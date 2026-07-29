import { fileURLToPath, URL } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

const repositoryRoot = fileURLToPath(new URL('../..', import.meta.url));
const defaultAPIOrigin = 'http://127.0.0.1:8080';

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, repositoryRoot, '');
  const apiOrigin =
    process.env.STACKPLORE_DEV_API_ORIGIN ||
    environment.STACKPLORE_DEV_API_ORIGIN ||
    defaultAPIOrigin;

  return {
    plugins: [react(), tailwindcss()],
    server: {
      host: '127.0.0.1',
      port: 5173,
      proxy: {
        '/api': {
          target: apiOrigin,
        },
        '/healthz': {
          target: apiOrigin,
        },
      },
      strictPort: true,
    },
  };
});
