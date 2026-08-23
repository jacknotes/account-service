import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// base 固定为 /app/，与后端 Gin 的静态托管路径一致
export default defineConfig({
  plugins: [vue()],
  base: '/app/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8081',
    },
  },
})
