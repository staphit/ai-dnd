import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  root: '.',
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../web-dist',
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            // Keep the large three.js / react-three stack in its own async chunk
            // (this group must precede the node_modules catch-all, else three is
            // pulled into the eager vendor chunk and the DMTable lazy-load is moot).
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
    proxy: {
      '/api': 'http://127.0.0.1:4318',
      '/generated': 'http://127.0.0.1:4318',
    },
  },
});
