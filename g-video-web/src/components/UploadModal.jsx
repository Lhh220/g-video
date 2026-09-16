import React, { useState } from 'react';
import axios from 'axios';
import SparkMD5 from 'spark-md5';

const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB 一片
const MAX_RETRY = 3; // 单片失败重试次数

const authHeaders = () => ({ Authorization: `Bearer ${localStorage.getItem('token')}` });

// 增量计算整个文件的 MD5，避免大文件一次性读进内存
async function computeMD5(file, onProgress) {
  const spark = new SparkMD5.ArrayBuffer();
  const chunks = Math.ceil(file.size / CHUNK_SIZE);
  for (let i = 0; i < chunks; i++) {
    const buf = await file.slice(i * CHUNK_SIZE, (i + 1) * CHUNK_SIZE).arrayBuffer();
    spark.append(buf);
    onProgress && onProgress(Math.round(((i + 1) / chunks) * 100));
  }
  return spark.end();
}

const UploadModal = ({ isOpen, onClose }) => {
  const [title, setTitle] = useState('');
  const [file, setFile] = useState(null);
  const [phase, setPhase] = useState('idle'); // idle | hashing | uploading | merging
  const [percent, setPercent] = useState(0);
  const [busy, setBusy] = useState(false);

  const handleUpload = async () => {
    if (!title || !file) return alert("请填写标题并选择视频");
    setBusy(true);
    try {
      // 1. 计算文件指纹 (秒传/断点续传的识别钥匙)
      setPhase('hashing');
      setPercent(0);
      const md5 = await computeMD5(file, setPercent);

      // 2. 初始化：后端判断 秒传 / 续传 / 新会话
      setPhase('uploading');
      setPercent(0);
      const progressKey = `upload:${md5}`;
      const saved = JSON.parse(localStorage.getItem(progressKey) || 'null');

      const initRes = await axios.post('/api/v1/video/upload/init', {
        filename: file.name,
        file_size: file.size,
        file_md5: md5,
        prev_upload_id: saved?.uploadId || ''
      }, { headers: authHeaders() });
      if (initRes.data.status_code !== 0) throw new Error(initRes.data.status_msg || '初始化上传失败');

      let uploadId = '';
      if (!initRes.data.uploaded) {
        uploadId = initRes.data.upload_id;
        // 记录会话 ID，页面刷新/断网后可续传
        localStorage.setItem(progressKey, JSON.stringify({ uploadId }));

        // 断点续传：跳过后端已确认收到的分片
        const done = new Set(initRes.data.existed_parts || []);
        const total = Math.ceil(file.size / CHUNK_SIZE);

        for (let i = 1; i <= total; i++) {
          if (done.has(i)) continue;
          const blob = file.slice((i - 1) * CHUNK_SIZE, i * CHUNK_SIZE);

          let ok = false;
          for (let r = 0; r < MAX_RETRY && !ok; r++) {
            try {
              const fd = new FormData();
              fd.append('upload_id', uploadId);
              fd.append('part_number', i);
              fd.append('data', blob);
              const res = await axios.post('/api/v1/video/upload/part', fd, { headers: authHeaders() });
              ok = res.data.status_code === 0;
            } catch { /* 网络抖动，重试 */ }
          }
          if (!ok) throw new Error('分片上传失败，请稍后重试 (已传分片会自动保留)');

          setPercent(Math.round((i / total) * 100));
        }
      }

      // 3. 完成：合并分片并发布 (秒传时 upload_id 留空，后端按 md5 复用云端文件)
      setPhase('merging');
      const cRes = await axios.post('/api/v1/video/upload/complete', {
        upload_id: uploadId,
        file_md5: md5,
        title
      }, { headers: authHeaders() });
      if (cRes.data.status_code !== 0) throw new Error(cRes.data.status_msg || '发布失败');

      localStorage.removeItem(progressKey);
      alert("发布成功！等待管理员审核");

      setTitle('');
      setFile(null);
      setPercent(0);
      onClose();
    } catch (err) {
      alert('上传失败: ' + (err.message || '未知错误'));
    } finally {
      setPhase('idle');
      setBusy(false);
    }
  };

  if (!isOpen) return null;

  const phaseText = {
    hashing: `计算文件指纹... ${percent}%`,
    uploading: `分片上传中... ${percent}%`,
    merging: '合并分片并发布...'
  };

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[100]">
      <div className="bg-zinc-900 p-8 rounded-2xl w-[400px] border border-zinc-700">
        <h2 className="text-white text-xl mb-4">上传视频</h2>
        <input
          className="w-full p-2 mb-4 bg-zinc-800 text-white rounded"
          placeholder="视频标题"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <input
          type="file"
          accept="video/*"
          className="text-zinc-400 mb-6"
          disabled={busy}
          onChange={(e) => setFile(e.target.files[0])}
        />

        {busy && (
          <div className="mb-6">
            <div className="text-zinc-400 text-sm mb-1">{phaseText[phase]}</div>
            <div className="w-full h-2 bg-zinc-800 rounded-full overflow-hidden">
              <div
                className="h-full bg-red-500 transition-all duration-300"
                style={{ width: `${phase === 'merging' ? 100 : percent}%` }}
              />
            </div>
          </div>
        )}

        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="text-zinc-400" disabled={busy}>取消</button>
          <button
            onClick={handleUpload}
            disabled={busy}
            className="bg-red-500 text-white px-6 py-2 rounded-full font-bold disabled:opacity-50"
          >
            {busy ? '上传中...' : '发布'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default UploadModal;
