import React, { useState, useEffect, useCallback } from 'react';
import axios from 'axios';

// 管理员审核后台：展示待审核视频，支持通过 / 驳回
const Admin = () => {
  const [list, setList] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actingId, setActingId] = useState(null);

  const fetchList = useCallback(async (p = 1) => {
    setLoading(true);
    try {
      const res = await axios.get('/api/v1/admin/pending/list', {
        params: { page: p, page_size: 20 },
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
      });
      if (res.data.status_code === 0) {
        setList(res.data.video_list || []);
        setTotal(res.data.total || 0);
        setPage(p);
        setError('');
      } else {
        setError(res.data.status_msg || '加载失败');
      }
    } catch (err) {
      const msg = err.response?.status === 403
        ? '无权限：只有管理员可以访问本页'
        : '无法连接后端服务';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchList(1);
  }, [fetchList]);

  const handleAudit = async (videoId, action) => {
    let reason = '';
    if (action === 2) {
      reason = window.prompt('请输入驳回原因:');
      if (reason === null) return; // 用户取消
    }
    setActingId(videoId);
    try {
      const res = await axios.post(
        `/api/v1/admin/audit?video_id=${videoId}&action=${action}&reason=${encodeURIComponent(reason)}`,
        {},
        { headers: { Authorization: `Bearer ${localStorage.getItem('token')}` } }
      );
      if (res.data.status_code === 0) {
        // 从列表中移除已处理项
        setList(prev => prev.filter(v => v.id !== videoId));
        setTotal(t => Math.max(0, t - 1));
      } else {
        alert('操作失败: ' + (res.data.status_msg || '未知错误'));
      }
    } catch {
      alert('网络错误，操作失败');
    } finally {
      setActingId(null);
    }
  };

  const totalPages = Math.max(1, Math.ceil(total / 20));

  return (
    <div className="h-full w-full overflow-y-auto bg-black text-white">
      <div className="max-w-4xl mx-auto p-8">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-3xl font-bold">🛡️ 视频审核</h1>
          <span className="text-zinc-400">待处理: {total} 条</span>
        </div>

        {loading && <p className="text-zinc-400 text-center py-20">加载中...</p>}

        {!loading && error && (
          <div className="text-center py-20 text-red-400">{error}</div>
        )}

        {!loading && !error && list.length === 0 && (
          <div className="text-center py-20 text-zinc-400">
            🎉 没有待审核的视频
          </div>
        )}

        {!loading && !error && list.map(v => (
          <div
            key={v.id}
            className="flex gap-4 bg-zinc-900 border border-zinc-800 rounded-2xl p-4 mb-4"
          >
            {/* 视频预览：审核前先看内容 */}
            <video
              src={v.play_url}
              poster={v.cover_url}
              controls
              preload="metadata"
              className="w-64 h-40 bg-black rounded-lg object-contain shrink-0"
            />

            <div className="flex-1 flex flex-col justify-between min-w-0">
              <div>
                <h3 className="text-xl font-bold truncate">{v.title || '(无标题)'}</h3>
                <p className="text-zinc-400 mt-1">
                  UP主: {v.author?.username || '未知用户'} · ❤️ {v.favorite_count} · 💬 {v.comment_count}
                </p>
              </div>

              <div className="flex gap-3 justify-end">
                <button
                  onClick={() => handleAudit(v.id, 2)}
                  disabled={actingId === v.id}
                  className="px-5 py-2 rounded-full border border-red-500 text-red-400 font-bold hover:bg-red-500 hover:text-white transition disabled:opacity-50"
                >
                  驳回
                </button>
                <button
                  onClick={() => handleAudit(v.id, 1)}
                  disabled={actingId === v.id}
                  className="px-5 py-2 rounded-full bg-green-600 text-white font-bold hover:bg-green-500 transition disabled:opacity-50"
                >
                  通过
                </button>
              </div>
            </div>
          </div>
        ))}

        {/* 分页 */}
        {!loading && !error && totalPages > 1 && (
          <div className="flex items-center justify-center gap-4 mt-6">
            <button
              onClick={() => fetchList(page - 1)}
              disabled={page <= 1}
              className="px-4 py-2 rounded-full bg-zinc-800 disabled:opacity-40"
            >
              上一页
            </button>
            <span className="text-zinc-400">{page} / {totalPages}</span>
            <button
              onClick={() => fetchList(page + 1)}
              disabled={page >= totalPages}
              className="px-4 py-2 rounded-full bg-zinc-800 disabled:opacity-40"
            >
              下一页
            </button>
          </div>
        )}
      </div>
    </div>
  );
};

export default Admin;
