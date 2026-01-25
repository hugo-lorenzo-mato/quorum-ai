import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/web/dist',
    emptyDirBefore: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/sse': 'http://localhost:8080',
    },
  },
})
