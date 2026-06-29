/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The board panel is a static SPA embedded into the Go binary
// (architecture/distribution-packaging.md). In dev, Vite proxies /api to the
// local `vector serve` process so the frontend talks to the real board API.
const API_TARGET = process.env.VECTOR_API ?? 'http://127.0.0.1:8787'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: API_TARGET, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'happy-dom',
  },
})
