(() => {
  const root = globalThis.document.documentElement;
  let locale = 'en';

  try {
    const storedLocale = globalThis.localStorage.getItem('stackplore.locale');
    if (storedLocale === 'en' || storedLocale === 'fa') {
      locale = storedLocale;
    }
  } catch {
    locale = 'en';
  }

  root.lang = locale;
  root.dir = locale === 'fa' ? 'rtl' : 'ltr';
  root.dataset.locale = locale;
})();
