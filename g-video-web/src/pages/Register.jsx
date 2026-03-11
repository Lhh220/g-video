import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import axios from 'axios';

const Register = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState('user'); // 默认字符串，发送时转数字
  const navigate = useNavigate();

  const handleRegister = async () => {
    if (!username || !password) {
      alert("请完整填写用户名和密码");
      return;
    }

    // --- 统一逻辑：把字符串角色转为后端需要的数字 0 或 1 ---
    const roleNum = role === 'admin' ? 1 : 0;

    try {
      const res = await axios.post('/api/v1/user/register', {
        username,
        password,
        role: roleNum
      });

      if (res.data.status_code === 0) {
        alert("注册成功！请前往登录");
        navigate('/login');
      } else {
        alert(res.data.status_msg || "注册失败");
      }
    } catch (err) {
      console.error(err);
      alert("注册失败，请检查后端服务");
    }
  };

  return (
    <div className="h-screen w-full flex items-center justify-center bg-black">
      
      {/* 注册卡片 - 宽度高度与登录页完全一致 */}
      <div className="w-[650px] h-[600px] bg-white/10 backdrop-blur-md border border-white/20 rounded-2xl shadow-xl">
        
        {/* 垂直居中布局 */}
        <div className="h-full flex flex-col justify-center px-16">
          
          {/* 标题 */}
          <h1 className="text-5xl font-bold text-white text-center mb-12">
            g-video短视频系统注册
          </h1>

          {/* 用户名输入框 */}
          <div className="flex justify-center mb-8">
            <div className="relative w-[300px]">
              <input
                className="w-full h-[50px] pl-10 pr-6 bg-gray-200 rounded-full outline-none text-xl placeholder:text-gray-500"
                placeholder="请设置用户名"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>
          </div>

          {/* 密码输入框 */}
          <div className="flex justify-center mb-8">
            <div className="relative w-[300px]">
              <input
                type="password"
                className="w-full h-[50px] pl-10 pr-6 bg-gray-200 rounded-full outline-none text-xl placeholder:text-gray-500"
                placeholder="请设置密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
          </div>

          {/* 角色选择 - Radio样式完全一致 */}
          <div className="flex justify-center mb-8">
            <div className="flex items-center gap-12 text-white text-xl">
              <span className="text-2xl font-medium">身份：</span>
              <label className="flex items-center gap-3 cursor-pointer hover:text-purple-300 transition">
                <input
                  type="radio"
                  name="role"
                  value="admin"
                  checked={role === 'admin'}
                  onChange={(e) => setRole(e.target.value)}
                  className="w-5 h-5"
                />
                管理员
              </label>
              <label className="flex items-center gap-3 cursor-pointer hover:text-purple-300 transition">
                <input
                  type="radio"
                  name="role"
                  value="user"
                  checked={role === 'user'}
                  onChange={(e) => setRole(e.target.value)}
                  className="w-5 h-5"
                />
                普通用户
              </label>
            </div>
          </div>

          {/* 注册按钮 - 颜色稍微区分或保持一致均可，这里保持一致 */}
          <div className="flex justify-center mb-8">
            <button
              onClick={handleRegister}
              className="w-[300px] h-[50px] rounded-full bg-gray-300 text-black text-2xl font-semibold hover:bg-gray-400 transition"
            >
              注 册
            </button>
          </div>

          {/* 返回登录链接 */}
          <div className="text-center text-white text-xl">
            已有账号？
            <Link
              to="/login"
              className="text-purple-400 ml-2 hover:text-purple-300 hover:underline transition"
            >
              返回登录 →
            </Link>
          </div>
          
        </div>
      </div>
    </div>
  );
};

export default Register;