import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

const createMediaQueryList = (query: string): MediaQueryList => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener: vi.fn(),
  addListener: vi.fn(),
  dispatchEvent: vi.fn(() => false),
  removeEventListener: vi.fn(),
  removeListener: vi.fn(),
});

Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: vi.fn(createMediaQueryList),
  writable: true,
});

Object.defineProperty(globalThis, 'scrollTo', {
  configurable: true,
  value: vi.fn(),
  writable: true,
});

class TestResizeObserver {
  disconnect() {}

  observe() {}

  unobserve() {}
}

vi.stubGlobal('ResizeObserver', TestResizeObserver);

Object.defineProperties(HTMLElement.prototype, {
  hasPointerCapture: {
    configurable: true,
    value: vi.fn(() => false),
  },
  releasePointerCapture: {
    configurable: true,
    value: vi.fn(),
  },
  scrollIntoView: {
    configurable: true,
    value: vi.fn(),
  },
  setPointerCapture: {
    configurable: true,
    value: vi.fn(),
  },
});

afterEach(() => {
  cleanup();
  globalThis.localStorage.clear();
  globalThis.document.documentElement.lang = 'en';
  globalThis.document.documentElement.dir = 'ltr';
  delete globalThis.document.documentElement.dataset.locale;
  delete globalThis.document.documentElement.dataset.theme;
  delete globalThis.document.documentElement.dataset.themePreference;
  globalThis.document.documentElement.style.removeProperty('color-scheme');
  globalThis.document.title = '';
  const description = globalThis.document.querySelector<HTMLMetaElement>(
    'meta[name="description"]',
  );
  description?.remove();
});
