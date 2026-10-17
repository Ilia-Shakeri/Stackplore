import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/browser',
  testMatch: process.env.SP_BROWSER_MODE === 'normal' ? 'normal-auth.spec.ts' : 'release.spec.ts',
  workers: 1,
  outputDir: process.env.SP_BROWSER_OUTPUT ?? './test-results',
  timeout: 180_000,
  reporter: [
    ['list'],
    ['json', { outputFile: process.env.SP_BROWSER_RESULTS ?? '../../.tmp/browser-results.json' }],
  ],
  use: {
    baseURL: process.env.SP_BROWSER_ORIGIN ?? 'http://127.0.0.1:58097',
    headless: true,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
    launchOptions: process.env.SP_BROWSER_EXECUTABLE
      ? { executablePath: process.env.SP_BROWSER_EXECUTABLE }
      : {},
  },
});
