import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    cors: { origin: 'http://127.0.0.1:5173' },
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9090',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyRequest, request) => {
            const origin = request.headers.origin;
            if (origin !== undefined) proxyRequest.setHeader('Origin', origin);
          });
        },
      },
    },
  },
  test: {
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    restoreMocks: true,
  },
});
