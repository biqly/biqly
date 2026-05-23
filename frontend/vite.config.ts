import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'node',
  },
  server: {
    port: 3333,
    proxy: {
      '/api': {
        target: 'http://localhost:8888',
        changeOrigin: true,
        // Long AI routes (metadata describe, embeddings); align with nginx proxy_read_timeout
        timeout: 650_000,
        proxyTimeout: 650_000,
      },
    },
  },
})
