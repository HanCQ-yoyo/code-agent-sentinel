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
          monaco: ['monaco-editor'],
          vendor: ['react', 'react-dom', 'react-router-dom', 'zustand', 'react-markdown', 'remark-gfm'],
        },
      },
    },
  },
  server: { proxy: { '/api': 'http://127.0.0.1:41999' } },
})
