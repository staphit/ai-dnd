import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  root: '.',
  plugins: [react(), tailwindcss()],
  assetsInclude: ['**/*.glb'],
  resolve: { dedupe: ['three'] },
  build: {
    outDir: '../web-dist',
    emptyOutDir: true,
    // The intentionally isolated Three.js renderer is ~1 MB minified; keep
    // warnings focused on accidental growth in the eager application chunks.
    chunkSizeWarningLimit: 1100,
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            // Geometric DMTable pulls three.js; keep it out of the eager vendor chunk.
            { name: 'three', test: /node_modules\/(three|@react-three)/ },
            { name: 'vendor', test: /node_modules/ },
          ],
        },
      },
    },
  },
  server: {
    host: '127.0.0.1',
    port: 4317,
    fs: {
      // tokens.css lives at the monorepo root (Hallmark design system).
      allow: ['..'],
    },
    proxy: {
      '/api': 'http://127.0.0.1:4318',
      '/generated': 'http://127.0.0.1:4318',
    },
  },
});
