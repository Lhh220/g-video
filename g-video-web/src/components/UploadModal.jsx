import React, { useState } from 'react';
import axios from 'axios';

const UploadModal = ({ isOpen, onClose }) => {
  const [title, setTitle] = useState('');
  const [file, setFile] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleUpload = async () => {
    if (!title || !file) return alert("请填写标题并选择视频");
    
    const formData = new FormData();
    formData.append('title', title);
    formData.append('data', file); // 对应后端的 data 字段
    formData.append('token', localStorage.getItem('token')); // 如果后端需要 multipart 传 token

    setLoading(true);
    try {
     // ... 之前的 axios 请求代码 ...
      const res = await axios.post('/api/v1/video/publish', formData, {
        headers: { 
          'Content-Type': 'multipart/form-data',
          'Authorization': `Bearer ${localStorage.getItem('token')}` 
        }
      });

      // --- 修改这里 ---
      // 只要 status_code 是 0 或者 status_msg 包含 "成功"
      if (res.data.status_code === 0 || res.data.status_msg === "发布成功") {
        alert("发布成功！");
        
        // 成功后清空状态，方便下次上传
        setTitle('');
        setFile(null);
        
        // 执行关闭
        onClose(); 
      } else {
        // 如果后端报错了，把报错信息弹出来
        alert("发布失败: " + (res.data.status_msg || "未知错误"));
      }
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[100]">
      <div className="bg-zinc-900 p-8 rounded-2xl w-[400px] border border-zinc-700">
        <h2 className="text-white text-xl mb-4">上传视频</h2>
        <input 
          className="w-full p-2 mb-4 bg-zinc-800 text-white rounded" 
          placeholder="视频标题"
          onChange={(e) => setTitle(e.target.value)}
        />
        <input 
          type="file" 
          accept="video/*"
          className="text-zinc-400 mb-6"
          onChange={(e) => setFile(e.target.files[0])}
        />
        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="text-zinc-400">取消</button>
          <button 
            onClick={handleUpload}
            disabled={loading}
            className="bg-red-500 text-white px-6 py-2 rounded-full font-bold"
          >
            {loading ? '上传中...' : '发布'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default UploadModal;