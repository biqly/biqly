import { defineConfig } from 'vitest/config'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [tailwindcss(), react()],
  resolve: {
    dedupe: ['immer', 'recharts'],
  },
  optimizeDeps: {
    include: ['recharts'],
  },
  build: {
    // Pin the output baseline instead of relying on Vite's shifting default.
    target: 'es2022',
    // Emit source maps but don't reference them from the bundle: usable for
    // error tracking, not exposed to end users.
    sourcemap: 'hidden',
    rollupOptions: {
      output: {
        // Isolate large, stable vendors so app-code changes don't bust their
        // long-term cache (recharts ~352 KB, react-dom, i18n locale data ~126 KB).
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('recharts') || id.includes('d3-') || id.includes('victory-vendor')) {
              return 'charts'
            }
            if (
              /[\\/]node_modules[\\/](react|react-dom|react-router|react-router-dom|scheduler)[\\/]/.test(
                id,
              )
            ) {
              return 'react-vendor'
            }
          }
          if (id.includes('/src/i18n/locales/')) {
            return 'i18n'
          }
        },
      },
    },
  },
  test: {
    environment: 'node',
  },
  server: {
    port: 3333,
    // Fail loudly instead of silently hopping to another port (which would break
    // the fixed gateway/proxy expectations).
    strictPort: true,
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
