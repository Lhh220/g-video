import React, { useState, useEffect } from 'react';
import axios from 'axios';

const CommentDrawer = ({ videoId, isOpen, onClose }) => {
  const [comments, setComments] = useState([]);
  const [inputText, setInputText] = useState("");
  const [loading, setLoading] = useState(false);

  // 1. 获取当前用户 ID (转为 String 方便后续比较)
  const currentUserId = localStorage.getItem('user_id'); 

  useEffect(() => {
    if (isOpen && videoId) {
      fetchComments();
    }
  }, [isOpen, videoId]);

  // 获取评论列表
  const fetchComments = async () => {
    setLoading(true);
    try {
      const token = localStorage.getItem('token');
      const res = await axios.get(`/api/v1/comment/list?video_id=${videoId}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      // 根据 Postman 截图，结构为 res.data.comment_list
      setComments(res.data.comment_list || []);
    } catch (err) {
      console.error("加载失败", err);
    } finally {
      setLoading(false);
    }
  };

  // 发送评论 (action_type=1)
  const handlePostComment = async () => {
    if (!inputText.trim()) return;
    const token = localStorage.getItem('token');
    
    try {
      // 匹配 Postman 截图中的参数格式
      const res = await axios.post(
        `/api/v1/comment/action?video_id=${videoId}&action_type=1&comment_text=${encodeURIComponent(inputText)}`,
        {},
        { headers: { 'Authorization': `Bearer ${token}` } }
      );

      if (res.data.status_msg === "success") {
        setInputText("");
        fetchComments(); // 成功后刷新列表
      }
    // eslint-disable-next-line no-unused-vars
    } catch (err) {
      alert("发送失败，请确认是否登录");
    }
  };

  // 删除评论 (action_type=2)
  const handleDeleteComment = async (commentId) => {
    if (!window.confirm("确定要删除这条评论吗？")) return;
    const token = localStorage.getItem('token');

    try {
      // 根据你的 Postman 截图
      // 删除需要传 video_id, action_type=2 以及 comment_id
      const res = await axios.post(
        `/api/v1/comment/action?video_id=${videoId}&action_type=2&comment_id=${commentId}`,
        {},
        { headers: { 'Authorization': `Bearer ${token}` } }
      );

      if (res.data.status_msg === "success") {
        // 界面实时移除该评论
        setComments(prev => prev.filter(c => c.id !== commentId));
      }
    // eslint-disable-next-line no-unused-vars
    } catch (err) {
      alert("删除失败");
    }
  };

  if (!isOpen) return null;

  return (
    <div className="absolute inset-0 bg-black/60 z-[100] flex items-end justify-center" onClick={onClose}>
      <div 
        className="w-full h-[65%] bg-[#121212] rounded-t-[2.5rem] p-8 flex flex-col shadow-[0_-10px_40px_rgba(0,0,0,0.5)] border-t border-white/10"
        onClick={(e) => e.stopPropagation()}
      >
        {/* 头部状态栏 */}
        <div className="flex justify-between items-center mb-8">
          <h4 className="text-2xl font-bold text-white tracking-tight">
            评论 <span className="text-gray-500 ml-2 text-lg">({comments.length})</span>
          </h4>
          <button onClick={onClose} className="text-gray-500 hover:text-white transition text-3xl p-2">✕</button>
        </div>

        {/* 评论滚动区域 */}
        <div className="flex-1 overflow-y-auto space-y-8 pr-2 custom-scrollbar">
          {loading ? (
            <div className="text-center text-gray-500 py-10 italic">加载中...</div>
          ) : comments.length > 0 ? (
            comments.map((item) => (
              <div key={item.id} className="flex gap-5 group items-start">
                <img 
                  src={item.user?.avatar || 'https://via.placeholder.com/150'} 
                  className="w-14 h-14 rounded-full border-2 border-white/5 object-cover flex-shrink-0"
                  alt="avatar"
                />
                <div className="flex-1 flex flex-col min-w-0">
                  <div className="flex justify-between items-center">
                    <span className="text-blue-400 font-bold text-base truncate">@{item.user?.username}</span>
                    <div className="flex items-center gap-4">
                      <span className="text-gray-600 text-xs">{item.create_date}</span>
                      
                      {/* 权限判断：只有该条评论的用户 ID 等于当前登录用户 ID 时显示删除标志 */}
                      {String(item.user?.id) === String(currentUserId) && (
                        <button 
                          onClick={() => handleDeleteComment(item.id)}
                          className="text-red-400 hover:text-red-400 text-xl opacity-0 group-hover:opacity-100  transition-all active:scale-75"
                          title="删除"
                        >
                          🗑️
                        </button>
                      )}
                    </div>
                  </div>
                  <p className="text-gray-200 mt-2 text-lg leading-relaxed break-words">{item.content}</p>
                </div>
              </div>
            ))
          ) : (
            <div className="text-center text-gray-600 py-10 italic">还没有人评论，快来抢沙发~</div>
          )}
        </div>

        {/* 底部固定发送区 */}
        <div className="mt-6 pt-6 border-t border-white/10 flex gap-4 items-center">
          <input 
            type="text" 
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handlePostComment()}
            placeholder="写下你的想法..." 
            className="flex-1 bg-white/5 rounded-2xl px-6 py-4 text-white outline-none border border-transparent focus:border-blue-500/50 transition-all"
          />
          <button 
            onClick={handlePostComment}
            disabled={!inputText.trim()}
            className="bg-blue-600 hover:bg-blue-500 disabled:bg-gray-800 disabled:text-gray-500 text-white px-8 py-4 rounded-2xl font-bold transition-all active:scale-95 flex-shrink-0"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  );
};

export default CommentDrawer;