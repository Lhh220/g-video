import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import Home from './pages/Home';
import AuthRoute from './components/AuthRoute';
import Sidebar from './components/Sidebar'; // 稍后创建这个组件

// 布局组件：左侧固定侧边栏，右侧渲染子页面
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
        {/* 公开路由 */}
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        
        {/* 受保护路由：全部包裹在 AuthRoute 和 MainLayout 中 */}
        <Route path="/" element={
          <AuthRoute>
            <MainLayout>
              <Home />
            </MainLayout>
          </AuthRoute>
        } />

        {/* 以后添加“我的”或“关注”页面只需在这里加 Route 即可 */}
        <Route path="/profile" element={
          <AuthRoute>
            <MainLayout>
              <div className="text-white p-10 text-3xl">我的个人页 (开发中)</div>
            </MainLayout>
          </AuthRoute>
        } />

      </Routes>
    </Router>
  );
}

export default App;