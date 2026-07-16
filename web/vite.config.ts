import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: 5173,
    proxy: {
      '/admin': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/me': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/account': 'http://localhost:8080',
    },
  },
})
