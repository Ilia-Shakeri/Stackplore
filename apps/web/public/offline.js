(() => {
  let preference = 'system';
  try {
    const stored = globalThis.localStorage.getItem('stackplore.theme');
    if (stored === 'light' || stored === 'dark' || stored === 'system') preference = stored;
  } catch {
    /* Unavailable storage falls back to the device preference. */
  }
  const systemIsDark = globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false;
  const theme = preference === 'system' ? (systemIsDark ? 'dark' : 'light') : preference;
  globalThis.document.documentElement.dataset.theme = theme;
})();

globalThis.document.getElementById('retry')?.addEventListener('click', () => {
  globalThis.location.reload();
});
