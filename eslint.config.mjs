import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';

const typescriptFiles = [
  'apps/web/**/*.{ts,tsx}',
  'packages/ui/**/*.{ts,tsx}',
  'packages/api-client/src/**/*.ts',
];
const strictTypeScript = tseslint.configs.strictTypeChecked.map((config) => ({
  ...config,
  files: typescriptFiles,
}));

export default tseslint.config(
  {
    ignores: ['**/dist/**', '**/node_modules/**', 'packages/api-client/src/derived/**'],
  },
  eslint.configs.recommended,
  ...strictTypeScript,
  {
    files: typescriptFiles,
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
      globals: {
        document: 'readonly',
      },
    },
    rules: {
      '@typescript-eslint/consistent-type-imports': 'error',
    },
  },
  {
    files: ['apps/web/tests/browser/**/*.ts', 'apps/web/playwright.config.ts'],
    languageOptions: {
      parserOptions: {
        projectService: false,
        project: './apps/web/tsconfig.browser.json',
      },
    },
  },
);
