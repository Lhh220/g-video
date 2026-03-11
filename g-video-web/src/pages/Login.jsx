import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import axios from 'axios';

const Login = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState('user');
  const navigate = useNavigate();

const handleLogin = async () => {
    if (!username || !password) {
      alert("请输入用户名和密码");
      return;
    }

    const roleNum = role === 'admin' ? 1 : 0; 

    try {
      const res = await axios.post('/api/v1/user/login', {
        username,
        password,
        role: roleNum
      });

      // 诊断：打印看看 res.data 到底长什么样
      console.log("后端返回原始数据:", res.data);

      // 修改判断逻辑：只要后端传回了 token，就代表登录成功
      if (res.data.token || res.data.status_code === 0) {
        
        // 强制存储
        localStorage.setItem('token', res.data.token);
        localStorage.setItem('user_id', res.data.user_id);
        localStorage.setItem('role', roleNum);

        console.log("验证存储结果:", localStorage.getItem('token'));

        alert("登录成功！");
        
        // 延迟跳转，确保存储生效
        setTimeout(() => {
          navigate('/');
        }, 100);
        
      } else {
        alert(res.data.status_msg || "登录失败：账号密码或角色错误");
      }
    } catch (err) {
      console.error("请求发生错误:", err);
      alert("无法连接到后端，请检查 Vite 代理或后端服务");
    }
  };

  return (
    <div className="h-screen w-full flex items-center justify-center bg-black">

      {/* 登录卡片 - 保持固定大小 */}
      <div className="w-[650px] h-[600px] bg-white/10 backdrop-blur-md border border-white/20 rounded-2xl shadow-xl">
        
        {/* 添加flex布局使内容垂直居中 */}
        <div className="h-full flex flex-col justify-center px-16">
          
          {/* 标题 */}
          <h1 className="text-5xl font-bold text-white text-center mb-12">
            g-video短视频系统登录
          </h1>

          {/* 用户名 - 增加左内边距 */}
          <div className="flex justify-center mb-8">
            <div className="relative w-[300px]">

              <input
                className="w-full h-[50px] pl-20 pr-6 bg-gray-200 rounded-full outline-none text-xl placeholder:text-gray-500"
                placeholder="请输入用户名"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>
          </div>

          {/* 密码 - 增加左内边距 */}
          <div className="flex justify-center mb-8">
            
            <div className="relative w-[300px]">

              <input
                type="password"
                className="w-full h-[50px] pl-20 pr-6 bg-gray-200 rounded-full outline-none text-xl placeholder:text-gray-500"
                placeholder="请输入密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
          </div>

          {/* 角色 */}
          <div className="flex justify-center mb-8">
            <div className="flex items-center gap-12 text-white text-xl">
              <span className="text-2xl font-medium">角色：</span>
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
                用户
              </label>
            </div>
          </div>

          {/* 登录按钮 */}
          <div className="flex justify-center mb-8">
            <button
              onClick={handleLogin}
              className="w-[300px] h-[50px] rounded-full bg-gray-300 text-black text-2xl font-semibold hover:bg-gray-400 transition"
            >
              登录
            </button>
          </div>

          {/* 注册链接 */}
          <div className="text-center text-white text-xl">
            没有账号？
            <Link
              to="/register"
              className="text-purple-400 ml-2 hover:text-purple-300 hover:underline transition"
            >
              点我快速注册 →
            </Link>
          </div>
          
        </div>
      </div>
    </div>
  );
};

export default Login;