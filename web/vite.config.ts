import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

const gatewayPrefix = '/app/fn-disk-wakeup-tracker/'

export default defineConfig({
  base: gatewayPrefix,
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(process.env.APP_VERSION || 'dev'),
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
})
