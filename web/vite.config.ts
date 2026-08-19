import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The API base URL is baked in at build time, which is why the portal image is
// rebuilt per environment rather than configured at runtime.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': '/src',
    },
  },
  build: { sourcemap: true },
  server: { host: true, port: 5173 },
});
