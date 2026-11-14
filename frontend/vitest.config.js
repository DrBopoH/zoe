import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// Компонентные тесты отдельно от node --test (там чистая логика без DOM): другой раннер, свой конфиг без tailwind.
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'happy-dom',
    include: ['tests/components/**/*.spec.js']
  }
})
