import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// When running inside Docker Compose, NGINX handles routing so the proxy
// is unused. When running `npm run dev` directly on the host, it forwards
// API calls to the gateway at localhost:8090.
const API_TARGET = process.env.VITE_API_TARGET ?? 'http://localhost:8090';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api':    { target: API_TARGET, changeOrigin: true },
      '/ingest': { target: API_TARGET, changeOrigin: true },
      '/data':   { target: API_TARGET, changeOrigin: true },
    },
  },
});
