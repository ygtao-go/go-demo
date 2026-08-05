import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig, loadEnv } from 'vite'

// 参考文档：https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  // 开发环境代理目标（默认转发到 Go 后端；可通过 .env.local 的 VITE_PROXY_TARGET 覆盖）
  const proxyTarget = env.VITE_PROXY_TARGET || 'http://localhost:8080'

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      open: false,
      // 开发环境代理：将 /api 与 /swagger 转发到 Go 后端，规避跨域
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true,
        },
        '/swagger': {
          target: proxyTarget,
          changeOrigin: true,
        },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: false,
      chunkSizeWarningLimit: 1500,
      rollupOptions: {
        output: {
          manualChunks: {
            vue: ['vue', 'vue-router', 'pinia'],
            element: ['element-plus', '@element-plus/icons-vue'],
            monaco: ['monaco-editor'],
            markdown: ['markdown-it'],
          },
        },
      },
    },
  }
})
