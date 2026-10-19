(() => {
  const storageKey = 'stackplore.theme';
  let preference = 'system';

  try {
    const stored = globalThis.localStorage.getItem(storageKey);
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      preference = stored;
    }
  } catch {
    preference = 'system';
  }

  const systemIsDark = globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false;
  const resolved = preference === 'system' ? (systemIsDark ? 'dark' : 'light') : preference;
  const root = globalThis.document.documentElement;

  root.dataset.theme = resolved;
  root.dataset.themePreference = preference;
  root.style.colorScheme = resolved;

  const themeColor = globalThis.document.querySelector('meta[name="theme-color"]');
  themeColor?.setAttribute('content', resolved === 'dark' ? '#06111f' : '#f4f8fd');
})();
