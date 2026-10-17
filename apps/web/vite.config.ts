import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath, URL } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv, type Plugin } from 'vite';

const repositoryRoot = fileURLToPath(new URL('../..', import.meta.url));
const defaultAPIOrigin = 'http://127.0.0.1:8080';

function publicShellVersion(): Plugin {
  return {
    name: 'public-offline-shell-version',
    apply: 'build',
    async writeBundle(options, bundle) {
      if (!options.dir) throw new Error('Public shell requires a build directory.');
      const digest = createHash('sha256');
      digest.update(await readFile(new URL('./package.json', import.meta.url)));
      for (const [name, output] of Object.entries(bundle).sort(([left], [right]) =>
        left.localeCompare(right),
      )) {
        digest.update(name);
        digest.update(output.type === 'chunk' ? output.code : output.source);
      }
      for (const name of [
        'offline.html',
        'offline.css',
        'offline.js',
        'brand/stackplore-favicon.svg',
      ]) {
        digest.update(await readFile(new URL(`./public/${name}`, import.meta.url)));
      }
      const template = await readFile(new URL('./public/sw.js', import.meta.url), 'utf8');
      digest.update(template);
      if (!template.includes('__STACKPLORE_RELEASE__')) {
        throw new Error('Public shell release marker is missing.');
      }
      await writeFile(
        resolve(options.dir, 'sw.js'),
        template.replace('__STACKPLORE_RELEASE__', digest.digest('hex').slice(0, 24)),
      );
    },
  };
}

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
    plugins: [react(), tailwindcss(), publicShellVersion()],
    server: {
      host: '127.0.0.1',
      port: 5173,
      proxy: {
        '/api': {
          changeOrigin: false,
          target: apiOrigin,
        },
        '/healthz': {
          changeOrigin: false,
          target: apiOrigin,
        },
        '/readyz': {
          changeOrigin: false,
          target: apiOrigin,
        },
        '/metrics': {
          changeOrigin: false,
          target: apiOrigin,
        },
      },
      strictPort: true,
    },
  };
});
