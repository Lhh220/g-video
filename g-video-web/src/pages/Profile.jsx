import React, { useEffect, useState } from 'react';
import axios from 'axios';
import { useNavigate } from 'react-router-dom'; // 新增：导入路由跳转
import EditProfileModal from '../components/EditProfileModal';
import VideoPlayerModal from '../components/VideoPlayerModal';

const Profile = () => {
  const [user, setUser] = useState(null);
  const [videos, setVideos] = useState([]);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [selectedVideo, setSelectedVideo] = useState(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate(); // 新增：初始化路由跳转

  // 新增：返回登录页方法
  const handleGoToLogin = () => {
    localStorage.removeItem('token'); // 清空无效token
    navigate('/login', { replace: true }); // 跳登录页，禁止返回
  };

  // 获取数据
const fetchProfileData = async () => {
  const token = localStorage.getItem('token');
  if (!token) {
    setLoading(false);
    return;
  }
  
  setLoading(true);
  try {
    // ========== 核心修改1：优先调用用户信息接口（确保能拿到 id） ==========
    // 先调用户信息接口，直接获取完整的用户数据（包含 id）
    const infoRes = await axios.get('/api/v1/user/info', {
      headers: { Authorization: `Bearer ${token}` }
    });
    
    console.log("用户信息接口返回：", infoRes.data); // 打印日志，排查返回结构
    if (infoRes.data.status_msg === "查询成功" && infoRes.data.user) {
      setUser(infoRes.data.user); // 直接赋值完整用户信息（包含 id）
    }

    // ========== 原有逻辑：调用投稿列表接口 ==========
    const res = await axios.get('/api/v1/video/publish/list', {
      headers: { Authorization: `Bearer ${token}` }
    });

    if (res.data.status_msg === "success") {
      const videoList = res.data.video_list || [];
      setVideos(videoList);
      
      // 可选：如果投稿列表的 author 信息更全，覆盖用户信息（保留你原有逻辑）
      if (videoList.length > 0 && videoList[0].author) {
        setUser(videoList[0].author);
      }
    }
  } catch (err) {
    console.error("加载失败", err);
    // ========== 核心修改2：接口报错时，单独重试用户信息接口 ==========
    try {
      const fallbackRes = await axios.get('/api/v1/user/info', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (fallbackRes.data.status_msg === "查询成功" && fallbackRes.data.user) {
        setUser(fallbackRes.data.user);
      }
    } catch (fallbackErr) {
      console.error("兜底获取用户信息失败：", fallbackErr);
      alert("获取用户信息失败，请重新登录");
      // 可选：清除无效 Token，跳转到登录页
      // localStorage.removeItem('token');
      // window.location.href = '/login';
    }
  } finally {
    setLoading(false);
  }
};

  // 删除视频逻辑
  const handleDeleteVideo = async (e, videoId) => {
    e.stopPropagation(); // 阻止触发父级的播放弹窗
    if (!window.confirm("确定要永久删除这段视频吗？")) return;

    const token = localStorage.getItem('token');
    try {
      // 匹配后端 RESTful 路由: DELETE /api/v1/video/:id
      const res = await axios.delete(`/api/v1/video/${videoId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (res.data.status_msg === "删除成功") {
        alert("删除成功");
        // 局部更新列表，不刷新页面
        setVideos(prev => prev.filter(v => v.id !== videoId));
      } else {
        alert("删除失败: " + res.data.status_msg);
      }
    // eslint-disable-next-line no-unused-vars
    } catch (err) {
      alert("网络错误，删除失败");
    }
  };

  useEffect(() => { fetchProfileData(); }, []);

  if (loading) return <div className="text-zinc-500 p-20 text-center">加载中...</div>;

  // 未登录状态：显示返回登录按钮
  if (!user) {
    return (
      <div className="flex-1 h-screen flex flex-col items-center justify-center bg-black text-white p-8">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold mb-2">未登录</h1>
          <p className="text-zinc-500">请先登录后查看个人主页</p>
        </div>
        {/* 新增：返回登录按钮 */}
        <button
          onClick={handleGoToLogin}
          className="px-8 py-3 bg-blue-600 hover:bg-blue-700 rounded-full text-sm font-bold transition-all"
        >
          返回登录
        </button>
      </div>
    );
  }

  return (
    <div className="flex-1 h-screen overflow-y-auto bg-black text-white p-8 custom-scrollbar relative">
      {/* 头部用户信息 */}
      <div className="flex items-center justify-between mb-12 pb-10 border-b border-white/10">
        <div className="flex items-center gap-8">
          <img 
            src={user?.avatar || 'https://via.placeholder.com/150'} 
            className="w-32 h-32 rounded-full object-cover border-4 border-zinc-800 shadow-2xl"
          />
          <div>
            <h1 className="text-4xl font-bold mb-2">{user?.username || '未登录'}</h1>
            <p className="text-zinc-500 font-mono text-sm">ID: {user?.id || '---'}</p>
            <div className="flex gap-6 mt-4 text-sm text-zinc-300">
              <span><strong className="text-white">{videos.length}</strong> 投稿</span>
              <span><strong className="text-white">{user?.follower_count || 0}</strong> 粉丝</span>
            </div>
          </div>
        </div>
        <button 
          onClick={() => setIsEditOpen(true)}
          className="px-6 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-full text-sm font-bold transition-all"
        >
          编辑资料
        </button>
      </div>

      {/* 视频列表网格 */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3">
        {videos.map((video) => (
          <div 
            key={video.id} 
            className="aspect-[3/4] relative cursor-pointer group overflow-hidden bg-zinc-900 rounded-lg"
            onClick={() => setSelectedVideo(video)}
          >
            {/* 封面 */}
            <img 
              src={video.cover_url} 
              className="w-full h-full object-cover group-hover:scale-110 transition duration-700" 
            />

            {/* 删除按钮 */}
            <button 
              onClick={(e) => handleDeleteVideo(e, video.id)}
              className="absolute top-2 right-2 p-2 bg-red-500/90 hover:bg-red-600 rounded-full opacity-0 group-hover:opacity-100 transition-all z-20 shadow-lg translate-y-2 group-hover:translate-y-0"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>

            {/* 底部信息遮罩 */}
            <div className="absolute inset-x-0 bottom-0 p-3 bg-gradient-to-t from-black/90 to-transparent">
              <p className="text-xs font-medium line-clamp-1">{video.title}</p>
            </div>
          </div>
        ))}
      </div>

      {/* 所有的弹窗都要放在外层，确保 z-index 不受干扰 */}
      <EditProfileModal 
        isOpen={isEditOpen} 
        user={user} 
        onClose={() => setIsEditOpen(false)} 
        onRefresh={fetchProfileData} 
      />
      
      {selectedVideo && (
        <VideoPlayerModal 
          video={selectedVideo} 
          onClose={() => setSelectedVideo(null)} 
        />
      )}
    </div>
  );
};

export default Profile;