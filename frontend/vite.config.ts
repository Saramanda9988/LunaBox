import {defineConfig} from 'vite'
import UnoCSS from 'unocss/vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
      react(),
      UnoCSS(),
  ],
  build: {
    minify: 'esbuild',
    target: 'es2020',
    rollupOptions: {
      output: {
        manualChunks: {
          // 将 html2canvas 单独分包（按需加载）
          'html2canvas': ['html2canvas'],
          // chart.js 相关
          'chart': ['chart.js', 'react-chartjs-2'],
        }
      }
    }
  }
})
