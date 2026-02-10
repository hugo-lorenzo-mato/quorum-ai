import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { configDefaults } from 'vitest/config'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../internal/web/dist',
    emptyDirBefore: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-editor': ['@monaco-editor/react'],
          'vendor-flow': ['reactflow'],
          'vendor-markdown': ['react-markdown', 'react-syntax-highlighter', 'remark-gfm', 'remark-breaks'],
          'vendor-icons': ['lucide-react'],
          'vendor-utils': ['zustand', 'turndown', 'clsx', 'tailwind-merge'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/sse': 'http://localhost:8080',
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.js',
    restoreMocks: true,
    exclude: [...configDefaults.exclude, 'e2e/**'],
    coverage: {
      reporter: ['text', 'html', 'lcov'],
    },
  },
})
