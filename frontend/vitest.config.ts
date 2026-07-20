import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  root: '.',
  plugins: [react()],
  assetsInclude: ['**/*.glb'],
  resolve: { dedupe: ['three'] },
  server: {
    fs: {
      // DMTable imports GLB clips from the monorepo root /glb (same as vite.config.ts).
      allow: ['..'],
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    restoreMocks: true,
  },
});
