import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import Home from './pages/Home';
import Follow from './pages/Follow'; // 1. 引入新创建的关注页面
import AuthRoute from './components/AuthRoute';
import Sidebar from './components/Sidebar';

// 布局组件
const MainLayout = ({ children }) => (
  <div className="flex h-screen w-full bg-black overflow-hidden">
    <Sidebar /> 
    <div className="flex-1 relative">
      {children}
    </div>
  </div>
);

function App() {
  return (
    <Router>
      <Routes>
        {/* --- 公开路由 --- */}
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        
        {/* --- 受保护路由：首页 --- */}
        <Route path="/" element={
          <AuthRoute>
            <MainLayout>
              <Home />
            </MainLayout>
          </AuthRoute>
        } />

        {/* 2. 新增受保护路由：关注页 */}
        <Route path="/follow" element={
          <AuthRoute>
            <MainLayout>
              <Follow />
            </MainLayout>
          </AuthRoute>
        } />

        {/* 3. 个人页占位（以后你也可以像 Home 一样把它独立成组件） */}
        <Route path="/profile" element={
          <AuthRoute>
            <MainLayout>
              <div className="text-white p-10 text-3xl font-bold italic">
                我的个人页 (COMING SOON...)
              </div>
            </MainLayout>
          </AuthRoute>
        } />

      </Routes>
    </Router>
  );
}

export default App;