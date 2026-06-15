import { defineConfig } from 'vitest/config'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [tailwindcss(), react()],
  resolve: {
    dedupe: ['immer', 'recharts'],
  },
  optimizeDeps: {
    include: ['recharts', 'es-toolkit', '@reduxjs/toolkit', 'immer'],
  },
  test: {
    environment: 'node',
  },
  server: {
    port: 3333,
    // Mirror the prod gateway: /api/auth/* is served by the auth service, every
    // other /api/* by the api. Order matters — Vite matches keys top-down, so the
    // more specific /api/auth must precede /api. No path rewrite (the auth service
    // mounts its routes under /api/auth, same as the gateway's PathPrefix).
    // NOTE: do NOT proxy /auth — that is a client-side SPA route (e.g.
    // /auth/signin is the sign-in page); it must fall through to index.html.
    proxy: {
      '/api/auth': {
        target: 'http://localhost:8889',
        changeOrigin: true,
      },
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
