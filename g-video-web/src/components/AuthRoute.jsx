import { Navigate } from 'react-router-dom';

// 原 AuthRoute：保护需要登录才能访问的页面（首页/关注页/个人页）
export const AuthRoute = ({ children }) => {
  const token = localStorage.getItem('token');
  
  if (!token) {
    // 未登录 → 强制跳转到登录页
    return <Navigate to="/login" replace />;
  }
  
  // 已登录 → 正常显示页面
  return children;
};

// 新增 GuestRoute：保护登录/注册页（已登录用户不能访问）
export const GuestRoute = ({ children }) => {
  const token = localStorage.getItem('token');
  
  if (token) {
    // 已登录 → 强制跳回首页，不让访问登录页
    return <Navigate to="/" replace />;
  }
  
  // 未登录 → 正常显示登录/注册页
  return children;
};

// 保留原默认导出（兼容你之前的代码）
export default AuthRoute;