import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        manualChunks: undefined
      }
    }
  },
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "@/styles/style.scss" as styles;`
      }
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '^(?!/api)/.*\\.(jpg|jpeg|png|gif|webp|svg|bmp|ico|heic|heif)$': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        bypass: (req) => {
          const url = req.url;
          if (url.startsWith('/src') || url.startsWith('/assets') || url.startsWith('/node_modules') || url.startsWith('/@')) {
            return url;
          }
          return null;
        }
      }
    }
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  optimizeDeps: {
    include: ['axios'] // 预构建常用依赖
  }
})