import react from '@vitejs/plugin-react';
import { configDefaults, defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    pool: 'threads',
    maxWorkers: 1,
    restoreMocks: true,
    setupFiles: ['./src/test/setup.ts'],
    testTimeout: 15_000,
    exclude: [...configDefaults.exclude, 'tests/browser/**'],
    coverage: {
      provider: 'v8',
      include: ['src/features/**/*.{ts,tsx}'],
      exclude: ['**/*.test.{ts,tsx}', '**/*test-fixtures*'],
      reporter: ['text-summary', 'json-summary'],
      thresholds: { statements: 70, branches: 70, functions: 70, lines: 70 },
    },
  },
});
