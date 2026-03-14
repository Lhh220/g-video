import React, { useState } from 'react'; // 👈 1. 引入 useState
import { useNavigate, useLocation } from 'react-router-dom';
import UploadModal from './UploadModal'; // 👈 2. 引入刚才创建的组件

const Sidebar = () => {
  const navigate = useNavigate();
  const location = useLocation();
  
  // 3. 定义弹窗显示状态
  const [isUploadOpen, setIsUploadOpen] = useState(false);

  const getIconStyle = (path) => 
    location.pathname === path ? "text-blue-400" : "text-gray-500 hover:text-white";

  return (
    <div className="w-32 bg-black border-r border-white/10 flex flex-col items-center py-12 justify-between z-50 h-screen shrink-0">
      <div className="space-y-16">
        <div onClick={() => navigate('/')} className={`flex flex-col items-center cursor-pointer transition ${getIconStyle('/')}`}>
          <span className="text-4xl">🏠</span>
          <span className="text-sm mt-2 font-bold tracking-widest">首页</span>
        </div>
        <div onClick={() => navigate('/follow')} className={`flex flex-col items-center cursor-pointer transition ${getIconStyle('/follow')}`}>
          <span className="text-4xl">👥</span>
          <span className="text-sm mt-2 font-bold tracking-widest">关注</span>
        </div>
      </div>

      {/* 4. 修改加号按钮的点击事件 */}
      <div 
        onClick={() => setIsUploadOpen(true)} // 👈 点击打开弹窗
        className="w-16 h-12 border-2 border-white rounded-xl flex items-center justify-center cursor-pointer hover:bg-white hover:text-black transition group"
      >
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

      {/* 5. 挂载弹窗组件 */}
      <UploadModal 
        isOpen={isUploadOpen} 
        onClose={() => setIsUploadOpen(false)} 
      />
    </div>
  );
};

export default Sidebar;