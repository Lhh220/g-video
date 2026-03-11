import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // 代理所有以 /api 开头的请求
      '/api': {
        target: 'http://localhost:8080', // 你的后端地址
        changeOrigin: true,              // 允许跨域转换
        // 如果你的后端接口路径里本来就有 /api，就保持这样
        // 如果后端接口没有 /api，请取消下面这一行的注释：
        // rewrite: (path) => path.replace(/^\/api/, '')
      }
    }
  }
})