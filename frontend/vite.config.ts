/// <reference types="vitest/config" />

/**
 * Vite 开发服务器、构建与 Vitest 配置；/admin/v1 和 /uploads 代理到本地 Go 服务。
 */
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // 开发环境把 API 与上传文件代理到本地 Go 服务，避免跨域。
    proxy: {
      '/admin/v1': 'http://127.0.0.1:8080',
      '/uploads': 'http://127.0.0.1:8080',
    },
  },
  build: {
    outDir: 'dist',
    // 生产构建不输出 sourcemap，避免暴露源码结构。
    sourcemap: false,
  },
  test: {
    // 组件测试运行在 jsdom 中，统一加载测试环境补丁。
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test/setup.ts'],
  },
})
