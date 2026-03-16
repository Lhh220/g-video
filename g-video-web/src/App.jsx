import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import Home from './pages/Home';
import Follow from './pages/Follow';
// 注意：这里要导入两个组件（AuthRoute + GuestRoute）
import { AuthRoute, GuestRoute } from './components/AuthRoute'; 
import Sidebar from './components/Sidebar';
import Profile from './pages/Profile';

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
        {/* 1. 登录/注册页：用 GuestRoute 保护 */}
        <Route path="/login" element={
          <GuestRoute>
            <Login />
          </GuestRoute>
        } />
        <Route path="/register" element={
          <GuestRoute>
            <Register />
          </GuestRoute>
        } />
        
        {/* 2. 需登录的页面：用 AuthRoute 保护 */}
        <Route path="/" element={
          <AuthRoute>
            <MainLayout>
              <Home />
            </MainLayout>
          </AuthRoute>
        } />
        <Route path="/follow" element={
          <AuthRoute>
            <MainLayout>
              <Follow />
            </MainLayout>
          </AuthRoute>
        } />
        <Route path="/profile" element={
          <AuthRoute>
            <MainLayout>
              <Profile />
            </MainLayout>
          </AuthRoute>
        } />
      </Routes>
    </Router>
  );
}

export default App;