import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  build: {
    rollupOptions: {
      output: {
        // Модель распознавания (tesseract.js) ищется по имени <язык>.traineddata.gz в каталоге: хеш в имени ломает поиск
        assetFileNames: (info) => ((info.names || [info.name || '']).some((n) => /\.traineddata\.gz$/.test(n)) ? 'ocr/[name][extname]' : 'assets/[name]-[hash][extname]')
      }
    }
  }
})
