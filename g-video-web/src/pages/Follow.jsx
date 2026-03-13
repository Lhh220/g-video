import React, { useState, useEffect, useRef } from 'react';
import axios from 'axios';
import CommentDrawer from '../components/CommentDrawer'; // 确保路径正确

const Follow = () => {
  const [videos, setVideos] = useState([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [loading, setLoading] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const [showComments, setShowComments] = useState(false);
  const videoRef = useRef(null);

  useEffect(() => {
  const fetchVideos = async () => {
    try {
      const token = localStorage.getItem('token');
      console.log("🛠️ 当前发送的 Token:", token); // 看看这里是不是 null 或者 "undefined"
      const config = token ? { headers: { 'Authorization': `Bearer ${token}` } } : {};
      
      const res = await axios.get('/api/v1/video/follow/feed', config);
      setVideos(res.data.video_list || []);
      setLoading(false);
    } catch (err) {
      console.error("加载视频失败:", err);
      setLoading(false);
    }
  };
  fetchVideos();
}, []);

  // 1. 关注/取关逻辑
  const handleFollow = async (authorId, isFollowed) => {
    const token = localStorage.getItem('token');
    if (!token) {
      alert("请先登录再关注哦！");
      return;
    }
    // 1为关注，2为取关
    const actionType = isFollowed ? 2 : 1;
    try {
      await axios.post(`/api/v1/relation/action?follow_user_id=${authorId}&action_type=${actionType}`, {}, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      
      // 更新本地状态，同步 UI
      const newVideos = [...videos];
      newVideos[currentIndex].author.is_follow = !isFollowed;
      setVideos(newVideos);
    // eslint-disable-next-line no-unused-vars
    } catch (err) {
      alert("关注操作失败，请重试");
    }
  };

  // 2. 点赞/取消点赞逻辑
  const handleFavorite = async (videoId, isFavorited) => {
    const token = localStorage.getItem('token');
    const actionType = isFavorited ? 2 : 1;
    try {
      await axios.post(`/api/v1/favorite/action?video_id=${videoId}&action_type=${actionType}`, {}, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      const newVideos = [...videos];
      newVideos[currentIndex].is_favorite = !isFavorited;
      setVideos(newVideos);
    // eslint-disable-next-line no-unused-vars
    } catch (err) {
      alert("点赞操作失败");
    }
  };

  const togglePlay = () => {
    if (videoRef.current) {
      if (videoRef.current.paused) {
        videoRef.current.play();
        setIsPaused(false);
      } else {
        videoRef.current.pause();
        setIsPaused(true);
      }
    }
  };

  const nextVideo = () => {
    if (currentIndex < videos.length - 1) {
      setCurrentIndex(currentIndex + 1);
      setIsPaused(false);
      setShowComments(false);
    }
  };

  const prevVideo = () => {
    if (currentIndex > 0) {
      setCurrentIndex(currentIndex - 1);
      setIsPaused(false);
      setShowComments(false);
    }
  };

  if (loading) return (
    <div className="flex-1 h-screen bg-black flex items-center justify-center text-white text-3xl font-bold italic tracking-widest">
      LOADING...
    </div>
  );

  const currentVideo = videos[currentIndex];

  return (
    <div className="flex-1 h-screen bg-[#0a0a0a] relative flex items-center justify-center overflow-hidden">
      
      {currentVideo ? (
        <div className="relative w-[85%] h-[85vh] bg-black rounded-[3rem] overflow-hidden shadow-[0_0_100px_rgba(0,0,0,0.5)] border border-white/5 group">
          
          <video
            ref={videoRef}
            key={currentVideo.play_url}
            src={currentVideo.play_url}
            className="w-full h-full object-contain cursor-pointer relative z-0"
            autoPlay
            loop
            onClick={togglePlay}
          />

          <CommentDrawer 
            videoId={currentVideo.id} 
            isOpen={showComments} 
            onClose={() => setShowComments(false)} 
          />

          <div className="absolute bottom-0 left-0 right-0 h-64 bg-gradient-to-t from-black/90 to-transparent z-10 pointer-events-none"></div>

          {/* 作者信息 */}
          <div className="absolute bottom-12 left-12 z-20 pointer-events-none text-white drop-shadow-2xl">
            <h3 className="text-4xl font-black mb-4 tracking-wide">
              @{currentVideo.author?.name || currentVideo.author?.username || '匿名用户'}
            </h3>
            <p className="text-2xl opacity-90 leading-relaxed max-w-[600px] line-clamp-2">
              {currentVideo.title}
            </p>
          </div>

          {/* 右侧互动按钮组 */}
          <div className="absolute right-10 bottom-24 z-20 flex flex-col gap-14 items-center">
            
            {/* 1. 作者头像与关注按钮 */}
            <div className="relative mb-4">
              <div className="w-20 h-20 rounded-full border-2 border-white overflow-hidden shadow-lg bg-gray-800">
                <img 
                  src={currentVideo.author?.avatar || 'https://via.placeholder.com/150'} 
                  alt="avatar" 
                  className="w-full h-full object-cover"
                />
              </div>
              {/* 关注按钮 */}
              <button 
                onClick={(e) => { 
                  e.stopPropagation(); 
                  handleFollow(currentVideo.author?.id, currentVideo.author?.is_follow); 
                }}
                className={`absolute -bottom-3 left-1/2 -translate-x-1/2 w-8 h-8 rounded-full flex items-center justify-center transition-all shadow-md active:scale-75
                  ${currentVideo.author?.is_follow ? 'bg-gray-500' : 'bg-red-500 hover:scale-110'}`}
              >
                <span className={`text-white text-2xl font-bold leading-none transition-transform ${currentVideo.author?.is_follow ? 'rotate-45' : 'rotate-0'}`}>
                  +
                </span>
              </button>
            </div>

            {/* 2. 点赞 */}
            <div 
              className="flex flex-col items-center gap-2 cursor-pointer transition transform hover:scale-110"
              onClick={(e) => { e.stopPropagation(); handleFavorite(currentVideo.id, currentVideo.is_favorite); }}
            >
              <span className={`text-7xl drop-shadow-lg transition-all active:scale-150 ${currentVideo.is_favorite ? 'text-red-500' : 'text-white'}`}>
                {currentVideo.is_favorite ? '❤️' : '🤍'}
              </span>
              <b className="text-white text-sm font-bold tracking-widest shadow-black">点赞</b>
            </div>

            {/* 3. 评论 */}
            <div 
              className="flex flex-col items-center gap-2 cursor-pointer transition transform hover:scale-110"
              onClick={(e) => { e.stopPropagation(); setShowComments(true); }}
            >
              <span className="text-7xl drop-shadow-lg">💬</span>
              <b className="text-white text-sm font-bold tracking-widest shadow-black">评论</b>
            </div>
          </div>

          {isPaused && (
            <div className="absolute inset-0 flex items-center justify-center bg-black/20 z-30 pointer-events-none">
              <span className="text-[140px] opacity-60">▶️</span>
            </div>
          )}
        </div>
      ) : (
        <div className="text-gray-500 text-3xl font-bold">未检测到视频流</div>
      )}

      {/* 翻页控制 */}
      <div className="fixed right-10 top-1/2 -translate-y-1/2 flex flex-col gap-12 z-[100]">
        <button onClick={prevVideo} className="w-24 h-24 bg-white/5 hover:bg-white/25 backdrop-blur-2xl rounded-full border-2 border-white/20 text-white flex items-center justify-center transition-all hover:scale-110">
          <span className="text-5xl font-light">▲</span>
        </button>
        <button onClick={nextVideo} className="w-24 h-24 bg-white/5 hover:bg-white/25 backdrop-blur-2xl rounded-full border-2 border-white/20 text-white flex items-center justify-center transition-all hover:scale-110">
          <span className="text-5xl font-light">▼</span>
        </button>
      </div>

    </div>
  );
};

export default Follow;