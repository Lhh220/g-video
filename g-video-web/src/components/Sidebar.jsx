import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';

const Sidebar = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const getIconStyle = (path) => 
    location.pathname === path ? "text-blue-400" : "text-gray-500 hover:text-white";

  return (
    // 宽度从 w-24 增加到 w-32
    <div className="w-32 bg-black border-r border-white/10 flex flex-col items-center py-12 justify-between z-20 h-screen shrink-0">
      <div className="space-y-16">
        {/* 图标从 text-3xl 增加到 text-4xl */}
        <div onClick={() => navigate('/')} className={`flex flex-col items-center cursor-pointer transition ${getIconStyle('/')}`}>
          <span className="text-4xl">🏠</span>
          <span className="text-sm mt-2 font-bold tracking-widest">首页</span>
        </div>
        <div onClick={() => navigate('/follow')} className={`flex flex-col items-center cursor-pointer transition ${getIconStyle('/follow')}`}>
          <span className="text-4xl">👥</span>
          <span className="text-sm mt-2 font-bold tracking-widest">关注</span>
        </div>
      </div>

      {/* 加号按钮变大 */}
      <div className="w-16 h-12 border-2 border-white rounded-xl flex items-center justify-center cursor-pointer hover:bg-white hover:text-black transition group">
        <span className="text-3xl font-bold">+</span>
      </div>

      <div className="space-y-16">
        <div onClick={() => navigate('/message')} className={`flex flex-col items-center cursor-pointer transition ${getIconStyle('/message')}`}>
          <span className="text-4xl">💬</span>
          <span className="text-sm mt-2 font-bold tracking-widest">消息</span>
        </div>
        <div onClick={() => navigate('/profile')} className={`flex flex-col items-center cursor-pointer transition ${getIconStyle('/profile')}`}>
          <span className="text-4xl">👤</span>
          <span className="text-sm mt-2 font-bold tracking-widest">我的</span>
        </div>
      </div>
    </div>
  );
};

export default Sidebar;