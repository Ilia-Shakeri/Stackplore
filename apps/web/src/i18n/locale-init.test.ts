/// <reference types="node" />

import { runInNewContext } from 'node:vm';

import { describe, expect, it } from 'vitest';

import documentSource from '../../index.html?raw';
import initializerSource from '../../public/locale-init.js?raw';
import { DEFAULT_LOCALE_STORAGE_KEY } from './locale';

interface BootRoot {
  dataset: Record<string, string>;
  dir: string;
  lang: string;
}

function runInitializer(storedLocale: string | null, storageError = false): BootRoot {
  const root: BootRoot = { dataset: {}, dir: '', lang: '' };

  runInNewContext(initializerSource, {
    globalThis: {
      document: { documentElement: root },
      localStorage: {
        getItem(key: string) {
          if (storageError) {
            throw new Error('Storage is unavailable.');
          }

          return key === DEFAULT_LOCALE_STORAGE_KEY ? storedLocale : null;
        },
      },
    },
  });

  return root;
}

describe('locale pre-paint initializer', () => {
  it('restores the shared Persian locale key and direction', () => {
    expect(runInitializer('fa')).toEqual({
      dataset: { locale: 'fa' },
      dir: 'rtl',
      lang: 'fa',
    });
  });

  it('uses English when storage is invalid or unavailable', () => {
    expect(runInitializer('invalid')).toEqual({
      dataset: { locale: 'en' },
      dir: 'ltr',
      lang: 'en',
    });
    expect(runInitializer(null, true)).toEqual({
      dataset: { locale: 'en' },
      dir: 'ltr',
      lang: 'en',
    });
  });

  it('loads before theme setup and the application entry module', () => {
    const localeIndex = documentSource.indexOf('<script src="/locale-init.js"></script>');
    const themeIndex = documentSource.indexOf('<script src="/theme-init.js"></script>');
    const applicationIndex = documentSource.indexOf(
      '<script type="module" src="/src/main.tsx"></script>',
    );

    expect(localeIndex).toBeGreaterThan(-1);
    expect(themeIndex).toBeGreaterThan(localeIndex);
    expect(applicationIndex).toBeGreaterThan(themeIndex);
  });
});
