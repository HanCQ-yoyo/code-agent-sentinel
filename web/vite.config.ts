import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 6000,
    rollupOptions: {
      output: {
        manualChunks: {
          antd: ['antd', '@ant-design/icons', '@ant-design/x'],
          // 不显式拆 monaco chunk:monaco-editor 仅被 MonacoViewer 动态 import(React.lazy),
          // Vite 自动把它拆成 dynamic chunk(按需加载),首屏 index.html 不再 modulepreload 它,
          // 回归 spec D2「markdown 默认预览不触发 Monaco 加载」(首屏 fetch 不含 ~3.7MB monaco)。
          vendor: ['react', 'react-dom', 'react-router-dom', 'zustand', 'react-markdown', 'remark-gfm'],
        },
      },
    },
  },
  server: { proxy: { '/api': 'http://127.0.0.1:41999' } },
})
