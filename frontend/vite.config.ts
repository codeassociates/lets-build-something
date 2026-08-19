import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    // The API is a separate service; proxying in development means the browser
    // sees one origin and the session cookie behaves exactly as it does in
    // production behind the reverse proxy.
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: { outDir: 'dist', sourcemap: true },
})
