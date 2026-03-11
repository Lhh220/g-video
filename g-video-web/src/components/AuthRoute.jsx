import { Navigate } from 'react-router-dom';

// 这个组件的作用是：检查有没有 Token
const AuthRoute = ({ children }) => {
  const token = localStorage.getItem('token');
  
  if (!token) {
    // 没登录？滚去登录页
    return <Navigate to="/login" replace />;
  }
  
  // 登录了？正常显示页面内容
  return children;
};

export default AuthRoute;