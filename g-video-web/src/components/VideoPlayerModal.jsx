import React from 'react';

const VideoPlayerModal = ({ video, onClose }) => {
  return (
    <div className="fixed inset-0 z-[1100] bg-black flex items-center justify-center">
      {/* 关闭按钮 */}
      <button 
        onClick={onClose}
        className="absolute top-10 left-10 text-white text-5xl z-[1200] hover:scale-110 transition"
      >
        ✕
      </button>

      {/* 播放器容器 */}
      <div className="relative w-full h-full flex items-center justify-center">
        <video 
          src={video.play_url}
          className="max-h-full max-w-full shadow-2xl"
          controls
          autoPlay
          loop
        />
        
        {/* 右侧或底部显示视频信息 */}
        <div className="absolute bottom-10 left-1/2 -translate-x-1/2 text-center bg-black/40 p-4 rounded-xl backdrop-blur-md">
          <h3 className="text-white font-bold text-xl">{video.title}</h3>
          <p className="text-zinc-400 text-sm mt-1">@{video.author.username}</p>
        </div>
      </div>
    </div>
  );
};

export default VideoPlayerModal;