import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';

const webFiles = ['apps/web/**/*.{ts,tsx}', 'packages/ui/**/*.{ts,tsx}'];
const strictTypeScript = tseslint.configs.strictTypeChecked.map((config) => ({
  ...config,
  files: webFiles,
}));

export default tseslint.config(
  {
    ignores: ['**/dist/**', '**/node_modules/**'],
  },
  eslint.configs.recommended,
  ...strictTypeScript,
  {
    files: webFiles,
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
);
