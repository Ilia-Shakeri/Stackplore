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
    build: {
      rolldownOptions: {
        output: {
          codeSplitting: {
            groups: [
              {
                name: 'react-runtime',
                test: /[\\/]node_modules[\\/](react|react-dom|scheduler)[\\/]/u,
              },
              { name: 'router-runtime', test: /[\\/]node_modules[\\/]@tanstack[\\/]/u },
              { name: 'ui-runtime', test: /[\\/]node_modules[\\/](@radix-ui|lucide-react)[\\/]/u },
            ],
          },
        },
      },
    },
    plugins: [react(), tailwindcss()],
    server: {
      host: '127.0.0.1',
      port: 5173,
      proxy: {
        '/api': {
          changeOrigin: true,
          target: apiOrigin,
        },
        '/healthz': {
          changeOrigin: true,
          target: apiOrigin,
        },
        '/readyz': {
          changeOrigin: true,
          target: apiOrigin,
        },
        '/metrics': {
          changeOrigin: true,
          target: apiOrigin,
        },
      },
      strictPort: true,
    },
  };
});
