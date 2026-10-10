import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

const backend = process.env.MEDIANINJA_BACKEND_URL ?? 'http://127.0.0.1:43189'
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: 5173, strictPort: true,
    proxy: { '/api': {
      target: backend, changeOrigin: true,
      configure(proxy) {
        // The trusted local dev proxy maintains the production same-origin API contract.
        proxy.on('proxyReq', req => { if (req.getHeader('origin')) req.setHeader('origin', backend) })
      },
    } },
  },
})
