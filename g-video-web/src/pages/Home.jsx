import React, { useState, useEffect, useRef } from 'react';
import axios from 'axios';

const Home = () => {
  const [videos, setVideos] = useState([]); 
  const [currentIndex, setCurrentIndex] = useState(0); 
  const [loading, setLoading] = useState(true);
  const [isPaused, setIsPaused] = useState(false); // 暂停状态
  const videoRef = useRef(null); // 视频引用

  useEffect(() => {
    const fetchVideos = async () => {
      try {
        const res = await axios.get('/api/v1/video/feed');
        setVideos(res.data.video_list || []);
        setLoading(false);
      } catch (err) {
        console.error("加载视频失败:", err);
        setLoading(false);
      }
    };
    fetchVideos();
  }, []);

  // 点击视频切换暂停/播放
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
    } catch (err) {
      alert("点赞操作失败");
    }
  };

  const nextVideo = () => {
    if (currentIndex < videos.length - 1) {
      setCurrentIndex(currentIndex + 1);
      setIsPaused(false); // 切换时重置为播放状态
    }
  };
  
  const prevVideo = () => {
    if (currentIndex > 0) {
      setCurrentIndex(currentIndex - 1);
      setIsPaused(false);
    }
  };

  if (loading) return <div className="h-full bg-black flex items-center justify-center text-white text-2xl font-bold">加载中...</div>;

  const currentVideo = videos[currentIndex];

  return (
    <div className="h-full w-full bg-black relative flex items-center justify-center overflow-hidden">
      
      {/* 1. 视频播放区域 */}
      {currentVideo ? (
        <div className="h-full w-full max-w-[500px] relative bg-gray-900 shadow-2xl cursor-pointer" onClick={togglePlay}>
          <video 
            ref={videoRef}
            key={currentVideo.play_url}
            src={currentVideo.play_url} 
            className="h-full w-full object-contain"
            autoPlay 
            loop
          />

          {/* 暂停时的图标提示 */}
          {isPaused && (
            <div className="absolute inset-0 flex items-center justify-center bg-black/20 z-10">
              <span className="text-white text-8xl opacity-70">▶️</span>
            </div>
          )}

          {/* 右下角互动 */}
          <div className="absolute bottom-32 right-6 flex flex-col items-center space-y-10 z-20" onClick={(e) => e.stopPropagation()}>
            <div onClick={() => handleFavorite(currentVideo.id, currentVideo.is_favorite)} className="cursor-pointer text-center">
              <div className={`text-5xl transition-all active:scale-150 ${currentVideo.is_favorite ? 'text-red-500' : 'text-white'}`}>
                {currentVideo.is_favorite ? '❤️' : '🤍'}
              </div>
              <span className="text-xs font-bold mt-2 block">点赞</span>
            </div>
            <div className="cursor-pointer text-center">
              <div className="text-5xl">💬</div>
              <span className="text-xs font-bold mt-2 block">评论</span>
            </div>
          </div>

          {/* 底部信息 */}
          <div className="absolute bottom-10 left-8 right-20 z-20" onClick={(e) => e.stopPropagation()}>
            <h3 className="font-black text-2xl mb-3">@{currentVideo.author?.name || '匿名用户'}</h3>
            <p className="text-lg text-gray-200 line-clamp-2 leading-relaxed">{currentVideo.title}</p>
          </div>
        </div>
      ) : (
        <div className="text-gray-500 text-2xl font-bold">暂无视频内容</div>
      )}

      {/* 2. 屏幕最右侧的上下切换按钮 */}
      <div className="fixed right-8 top-1/2 -translate-y-1/2 flex flex-col space-y-12 z-50">
        <button 
          onClick={prevVideo}
          className="w-20 h-20 bg-white/10 hover:bg-white/30 backdrop-blur-xl rounded-full flex items-center justify-center border-2 border-white/20 text-white transition-all active:scale-75 shadow-2xl"
        >
          <span className="text-4xl">▲</span>
        </button>
        <button 
          onClick={nextVideo}
          className="w-20 h-20 bg-white/10 hover:bg-white/30 backdrop-blur-xl rounded-full flex items-center justify-center border-2 border-white/20 text-white transition-all active:scale-75 shadow-2xl"
        >
          <span className="text-4xl">▼</span>
        </button>
      </div>

    </div>
  );
};

export default Home;