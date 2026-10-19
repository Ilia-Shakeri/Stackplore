/* Production build replaces the release marker with its content digest. */
const shellCache = 'stackplore-public-shell-__STACKPLORE_RELEASE__';
const shellPaths = new Map([
  ['/offline.html', 'text/html'],
  ['/offline.css', 'text/css'],
  ['/offline.js', 'javascript'],
  ['/brand/stackplore-favicon-64.png', 'image/png'],
]);

globalThis.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await globalThis.caches.open(shellCache);
      for (const [path, contentType] of shellPaths) {
        const request = new globalThis.Request(path, { cache: 'reload', credentials: 'omit' });
        const response = await globalThis.fetch(request);
        const policy = response.headers.get('Cache-Control') ?? '';
        if (
          !response.ok ||
          response.redirected ||
          !response.headers.get('Content-Type')?.includes(contentType) ||
          /(?:no-store|private)/iu.test(policy)
        ) {
          throw new Error('Public offline shell is unavailable.');
        }
        await cache.put(request, response);
      }
    })(),
  );
});

globalThis.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      for (const name of await globalThis.caches.keys()) {
        if (name.startsWith('stackplore-public-shell-') && name !== shellCache) {
          await globalThis.caches.delete(name);
        }
      }
      await globalThis.clients.claim();
    })(),
  );
});

globalThis.addEventListener('message', (event) => {
  if (event.data?.type === 'ACTIVATE_PUBLIC_SHELL') {
    event.waitUntil(globalThis.skipWaiting());
  }
});

globalThis.addEventListener('fetch', (event) => {
  const request = event.request;
  const url = new globalThis.URL(request.url);
  if (
    request.method !== 'GET' ||
    url.origin !== globalThis.location.origin ||
    url.pathname === '/api' ||
    url.pathname.startsWith('/api/') ||
    ['/healthz', '/readyz', '/metrics'].includes(url.pathname)
  ) {
    return;
  }
  if (request.mode === 'navigate') {
    event.respondWith(
      globalThis.fetch(request).catch(async () => {
        const cache = await globalThis.caches.open(shellCache);
        const shell = await cache.match('/offline.html');
        return shell ?? globalThis.Response.error();
      }),
    );
    return;
  }
  if (!url.search && shellPaths.has(url.pathname)) {
    event.respondWith(
      globalThis.caches.open(shellCache).then(async (cache) => {
        return (await cache.match(url.pathname)) ?? globalThis.fetch(request);
      }),
    );
  }
});
