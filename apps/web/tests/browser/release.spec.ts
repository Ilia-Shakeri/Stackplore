import { mkdir, readFile, unlink, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import { chromium, expect, test, type Page, type Request, type TestInfo } from '@playwright/test';
import axe from 'axe-core';

declare global {
  interface Window {
    axe: typeof axe;
  }
}

const version = process.env.SP_BROWSER_VERSION ?? '1.0.3';
const evidence = resolve('../../docs/evidence', version);
const defaultPeerBucketRefillMs = 2500;
const responseFailures = new WeakMap<Page, { component: string; status: number }[]>();
function refillDefaultPeerBucket(): Promise<void> {
  return new Promise((done) => {
    setTimeout(done, defaultPeerBucketRefillMs);
  });
}
function observeResponses(page: Page) {
  const failures: { component: string; status: number }[] = [];
  responseFailures.set(page, failures);
  page.on('response', (response) => {
    const path = new URL(response.url()).pathname;
    if (response.request().resourceType() === 'document' && response.status() >= 400)
      failures.push({ component: 'public-document', status: response.status() });
    if (path.startsWith('/assets/') && response.status() >= 400)
      failures.push({ component: 'public-assets', status: response.status() });
    if (path.startsWith('/api/v1/') && response.status() >= 400) {
      const component = path.split('/')[3] ?? 'unknown';
      if (/^[a-z-]+$/u.test(component)) failures.push({ component, status: response.status() });
    }
  });
}
test.beforeEach(async ({ page }) => {
  observeResponses(page);
  // Separate logical clients share one loopback peer and its default rate bucket.
  await refillDefaultPeerBucket();
});
test.afterEach(async ({ page }, info) => {
  if (info.status !== 'passed')
    await info.attach('public-http-statuses', {
      body: JSON.stringify(responseFailures.get(page) ?? []),
      contentType: 'application/json',
    });
});
async function receipt(key: string, value: unknown) {
  await mkdir(evidence, { recursive: true });
  const path = resolve(evidence, 'browser-measurements.json');
  let current: Record<string, unknown> = {};
  try {
    current = JSON.parse(await readFile(path, 'utf8')) as Record<string, unknown>;
  } catch {
    /* The first public fixture starts the sanitized receipt. */
  }
  current[key] = value;
  await writeFile(path, `${JSON.stringify(current, null, 2)}\n`);
}

async function readPublicWorker(page: Page, info: TestInfo): Promise<string> {
  const deadline = Date.now() + 15_000;
  const attempts: {
    attempt: number;
    status: number;
    mime: string;
    retryDelayMs?: number;
    bytes?: number;
    cacheMarkers?: number;
  }[] = [];
  try {
    for (let attempt = 1; attempt <= 4; attempt++) {
      const remaining = deadline - Date.now();
      expect(remaining, 'public worker control read deadline').toBeGreaterThan(0);
      const response = await page.request.get('/sw.js', { timeout: Math.min(5000, remaining) });
      try {
        const contentType = (response.headers()['content-type'] ?? '')
          .split(';')[0]
          ?.trim()
          .toLowerCase();
        const mime =
          contentType !== undefined &&
          ['text/javascript', 'application/javascript', 'text/plain'].includes(contentType)
            ? contentType
            : 'other';
        const observation: (typeof attempts)[number] = { attempt, status: response.status(), mime };
        attempts.push(observation);
        if (response.status() === 429) {
          expect(attempt, 'public worker control read retry count').toBeLessThan(4);
          const retryAfter = response.headers()['retry-after']?.trim();
          const requestedDelay =
            retryAfter === undefined || retryAfter.length > 128
              ? Number.NaN
              : /^\d{1,9}$/u.test(retryAfter)
                ? Number(retryAfter) * 1000
                : Date.parse(retryAfter) - Date.now();
          const delay = Math.max(0, requestedDelay);
          expect(
            Number.isFinite(delay) && delay <= 5000 && delay < deadline - Date.now(),
            'public worker control read requires a bounded Retry-After',
          ).toBe(true);
          observation.retryDelayMs = delay;
          await new Promise((done) => setTimeout(done, delay));
          continue;
        }
        expect(response.status(), 'public worker control read HTTP status').toBe(200);
        expect(
          ['text/javascript', 'application/javascript'].includes(mime),
          'public worker control read JavaScript MIME',
        ).toBe(true);
        const bytes = await response.body();
        observation.bytes = bytes.length;
        expect(bytes.length, 'public worker control read byte limit').toBeLessThanOrEqual(
          64 * 1024,
        );
        const source = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
        observation.cacheMarkers = (
          source.match(/(const shellCache = 'stackplore-public-shell-)[^']+(';)/gu) ?? []
        ).length;
        expect(observation.cacheMarkers, 'public worker control read cache declaration').toBe(1);
        return source;
      } finally {
        await response.dispose();
      }
    }
    throw new Error('Public worker control read retry budget exhausted.');
  } finally {
    await info.attach('public-worker-control-read', {
      body: JSON.stringify({ deadlineMs: 15_000, maximumAttempts: 4, attempts }),
      contentType: 'application/json',
    });
  }
}

async function verifyEnvironmentSelection(page: Page, locale: string, width: number) {
  await refillDefaultPeerBucket();
  let reads = 0;
  const observe = (request: Request) => {
    if (new URL(request.url()).pathname === '/api/v1/environments') reads++;
  };
  page.on('request', observe);
  try {
    const trigger = page.getByRole('button', {
      name: locale === 'fa' ? /^مرور محیط‌ها/u : /^Browse environments/u,
    });
    const dialog = page.getByRole('dialog', {
      name: locale === 'fa' ? 'انتخاب محیط' : 'Choose an environment',
    });
    await expect(trigger).toBeVisible();
    expect(reads, 'environment chooser stays lazy on About').toBe(0);
    await trigger.focus();
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('combobox')).toBeVisible();
    expect(reads).toBe(1);
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
    expect(
      await dialog.evaluate((element) => {
        const rect = element.getBoundingClientRect();
        return rect.left >= -1 && rect.right <= innerWidth + 1;
      }),
    ).toBe(true);
    const violations = await dialog.evaluate(async (element) => {
      const result = await window.axe.run(element, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
      });
      return result.violations.map(({ id }) => id);
    });
    expect(violations).toEqual([]);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
    await page.keyboard.press('Enter');
    const select = dialog.getByRole('combobox');
    await expect(select).toBeVisible();
    const choices = await select
      .locator('option:not(:disabled)')
      .evaluateAll((options) => options.map((option) => option.getAttribute('value') ?? ''));
    const value = choices.find((choice) => choice.startsWith('source_demo/'));
    expect(value, 'available synthetic environment').toBeDefined();
    if (value === undefined) throw new Error('Synthetic environment fixture is unavailable.');
    const [source, environment] = value.split('/');
    if (
      source === undefined ||
      environment === undefined ||
      !/^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(source) ||
      !/^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(environment)
    )
      throw new Error('Synthetic environment fixture identifier is invalid.');
    const resourceRead = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === '/api/v1/resources' &&
        url.searchParams.get('source_id') === source &&
        url.searchParams.get('environment_id') === environment
      );
    });
    await select.selectOption(value);
    expect((await resourceRead).status()).toBe(200);
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('main')).toBeFocused();
    const location = new URL(page.url());
    expect(location.pathname).toBe('/resources');
    expect(location.searchParams.get('source')).toBe(source);
    expect(location.searchParams.get('environment_id')).toBe(environment);
    await expect(
      page.getByRole('main').getByLabel(locale === 'fa' ? 'محیط' : 'Environment', { exact: true }),
    ).toHaveValue(environment);
    await expect(page.locator('.resource-list .resource-row').first()).toBeVisible();
    expect(reads).toBe(2);
    if (width === 360) {
      const openNavigation = page.getByRole('button', {
        name: locale === 'fa' ? 'باز کردن ناوبری' : 'Open navigation',
      });
      await openNavigation.click();
      const drawer = page.getByRole('dialog', { name: locale === 'fa' ? 'ناوبری' : 'Navigation' });
      await expect(drawer).toBeVisible();
      const events = drawer.getByRole('link', {
        name: locale === 'fa' ? 'رویدادها' : 'Events',
        exact: true,
      });
      const target = new URL((await events.getAttribute('href')) ?? '', page.url());
      expect(target.searchParams.get('source')).toBe(source);
      expect(target.searchParams.get('environment_id')).toBe(environment);
      await page.keyboard.press('Escape');
      await expect(drawer).toBeHidden();
      await expect(openNavigation).toBeFocused();
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(
      true,
    );
    return {
      lazyReads: true,
      environmentReads: reads,
      sourceBound: true,
      keyboard: true,
      focusReturn: true,
      accessibility: true,
      viewport: true,
      mobileDrawer: width === 360,
    };
  } finally {
    page.off('request', observe);
  }
}

for (const locale of ['en', 'fa']) {
  for (const width of [360, 1280]) {
    for (const theme of ['light', 'dark']) {
      test(`${locale} ${String(width)}px ${theme}: real guest journeys, accessibility and layout`, async ({
        page,
        browser,
      }) => {
        const measurements: unknown[] = [];
        await page.setViewportSize({ width, height: 900 });
        await page.emulateMedia({
          reducedMotion: 'reduce',
          colorScheme: theme === 'dark' ? 'dark' : 'light',
        });
        await page.addInitScript(
          ({ locale, theme }) => {
            localStorage.setItem('stackplore.locale', locale);
            localStorage.setItem('stackplore.theme', theme);
          },
          { locale, theme },
        );
        const errors: string[] = [];
        page.on('pageerror', (error) => errors.push(error.message));
        for (const route of [
          '/overview',
          '/resources',
          '/topology',
          '/logs',
          '/events',
          '/alerts',
          '/notifications',
          '/search',
        ]) {
          const started = Date.now();
          await page.goto(route);
          await expect(page.locator('main h1')).toBeVisible();
          if (route === '/resources' || route === '/search')
            await expect(page.locator('.resource-list .resource-row')).toHaveCount(9);
          if (route === '/logs')
            await expect(page.locator('.log-list article').first()).toBeVisible();
          if (route === '/topology')
            await expect(page.locator('.topology-canvas .topology-node')).toHaveCount(9);
          if (route === '/overview') {
            await expect(page.locator('.source-card').first()).toBeVisible();
            await expect(page.locator('.trend-card').first()).toBeVisible();
            await expect(page.locator('.trend-card svg polyline').first()).toBeVisible();
          }
          if (route === '/events')
            await expect(page.locator('.event-list article').first()).toBeVisible();
          if (route === '/alerts')
            await expect(page.locator('.alert-list article').first()).toBeVisible();
          if (route === '/notifications')
            await expect(page.locator('.notification-list article').first()).toBeVisible();
          const routeReadyMs = Date.now() - started;
          await expect(page.locator('html')).toHaveAttribute('lang', locale);
          await expect(page.locator('html')).toHaveAttribute(
            'dir',
            locale === 'fa' ? 'rtl' : 'ltr',
          );
          const timing = await page.evaluate(() => {
            const navigation = performance.getEntriesByType('navigation')[0] as
              PerformanceNavigationTiming | undefined;
            const resources = performance
              .getEntriesByType('resource')
              .filter((entry) =>
                new URL(entry.name).pathname.startsWith('/assets/'),
              ) as PerformanceResourceTiming[];
            return {
              domContentLoadedMs: Math.round(navigation?.domContentLoadedEventEnd ?? 0),
              loadMs: Math.round(navigation?.loadEventEnd ?? 0),
              assetCount: resources.length,
              assetTransferBytes: resources.reduce((sum, entry) => sum + entry.transferSize, 0),
              assetEncodedBytes: resources.reduce((sum, entry) => sum + entry.encodedBodySize, 0),
              reducedMotion: matchMedia('(prefers-reduced-motion: reduce)').matches,
              normalMotionDuration: getComputedStyle(document.documentElement)
                .getPropertyValue('--sp-duration-normal')
                .trim(),
            };
          });
          expect(timing.reducedMotion).toBe(true);
          expect(timing.normalMotionDuration).toBe('1ms');
          measurements.push({ route, routeReadyMs, ...timing });
          await page.evaluate(axe.source);
          const findings = await page.evaluate(async () => {
            const result = await window.axe.run(document, {
              runOnly: {
                type: 'tag',
                values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'],
              },
            });
            return result.violations.map(({ id, nodes }) => ({
              id,
              targets: nodes.map(({ target }) => target),
            }));
          });
          expect(findings, `${route} accessibility`).toEqual([]);
          expect(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
            `${route} overflow`,
          ).toBe(true);
          expect(Date.now() - started, `${route} readiness budget`).toBeLessThan(30_000);
          if (route === '/topology' || route === '/overview') {
            if (route === '/overview') {
              await expect(page.locator('.source-card').first()).toBeVisible();
              await expect(page.locator('.trend-card svg polyline').first()).toBeVisible();
            } else {
              await expect(page.locator('.topology-canvas .topology-node')).toHaveCount(9);
            }
            await page.screenshot({
              path: `${evidence}/screenshots/${route.slice(1)}-${locale}-${String(width)}-${theme}.png`,
              fullPage: true,
            });
          }
        }
        await page
          .getByRole('button', {
            name: locale === 'fa' ? 'بازکردن پالت فرمان' : 'Open command palette',
          })
          .click();
        const palette = page.getByRole('dialog', {
          name: locale === 'fa' ? 'رفتن به صفحه' : 'Go to a page',
        });
        await expect(palette.getByRole('combobox')).toBeFocused();
        const paletteFindings = await palette.evaluate(async (element) => {
          const result = await window.axe.run(element, {
            runOnly: {
              type: 'tag',
              values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'],
            },
          });
          return result.violations.map(({ id }) => id);
        });
        expect(paletteFindings).toEqual([]);
        expect(
          await palette.evaluate((element) => {
            const rect = element.getBoundingClientRect();
            return rect.left >= -1 && rect.right <= innerWidth + 1;
          }),
        ).toBe(true);
        await page.screenshot({
          path: `${evidence}/screenshots/command-palette-${locale}-${String(width)}-${theme}.png`,
          fullPage: true,
        });
        await palette.getByRole('combobox').fill('/about');
        await palette.getByRole('combobox').press('Enter');
        await expect(page).toHaveURL(/\/about(?:\?|$)/u);
        await expect(page.getByRole('main')).toBeFocused();
        const introduction = page.locator('.about-hero__image');
        await expect(introduction).toHaveAttribute('src', `/brand/stackplore-intro-${theme}.webp`);
        await introduction.scrollIntoViewIfNeeded();
        await expect
          .poll(() =>
            introduction.evaluate(
              (element) =>
                element instanceof HTMLImageElement &&
                element.complete &&
                element.naturalWidth === 960 &&
                element.naturalHeight === 640,
            ),
          )
          .toBe(true);
        const brandMark = page.locator('.brand-logo__mark').first();
        await expect(brandMark).toHaveAttribute('src', '/brand/stackplore-mark-96.webp');
        await expect
          .poll(() =>
            brandMark.evaluate(
              (element) =>
                element instanceof HTMLImageElement && element.complete && element.naturalWidth > 0,
            ),
          )
          .toBe(true);
        await page.screenshot({
          path: `${evidence}/screenshots/about-${locale}-${String(width)}-${theme}.png`,
          fullPage: true,
        });
        await page.keyboard.press('Control+k');
        await expect(palette).toBeVisible();
        await page.keyboard.press('Escape');
        await expect(page.getByRole('main')).toBeFocused();
        const environmentSelector =
          theme === 'light' ? await verifyEnvironmentSelection(page, locale, width) : undefined;
        expect(errors).toEqual([]);
        await receipt(`${locale}-${String(width)}-${theme}`, {
          scope: 'Loopback disposable synthetic guest fixture; local observations only.',
          version,
          chromium: browser.version(),
          measurements,
          commandPalette: {
            keyboard: true,
            accessibility: true,
            viewport: true,
            fixedNavigation: true,
            focusReturn: true,
          },
          ...(environmentSelector === undefined ? {} : { environmentSelector }),
          httpFailures: responseFailures.get(page) ?? [],
        });
      });
    }
  }
}

test('worker caches only public shell and offline navigation reveals no infrastructure', async ({
  page,
  context,
}) => {
  await page.goto('/overview');
  await expect(page.locator('main h1')).toBeVisible();
  expect((await page.request.get('/sw.js')).status()).toBe(200);
  await expect
    .poll(
      () =>
        page.evaluate(async () => (await navigator.serviceWorker.getRegistration())?.active?.state),
      { timeout: 15_000 },
    )
    .toBe('activated');
  await page.reload();
  await expect
    .poll(() => page.evaluate(() => navigator.serviceWorker.controller !== null))
    .toBe(true);
  const paths = await page.evaluate(async () => {
    const paths: string[] = [];
    for (const name of await caches.keys()) {
      for (const request of await (await caches.open(name)).keys())
        paths.push(new URL(request.url).pathname);
    }
    return paths.sort();
  });
  expect(paths).toEqual([
    '/brand/stackplore-favicon-64.png',
    '/offline.css',
    '/offline.html',
    '/offline.js',
  ]);
  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme === 'light' ? 'dark' : 'light' });
    await page.evaluate((selected) => {
      localStorage.setItem('stackplore.theme', selected);
    }, theme);
    await context.setOffline(true);
    await page.goto('/resources');
    await expect(page.getByRole('heading', { name: 'You are offline' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'اتصال شبکه قطع است' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await expect(page.locator('html')).toHaveCSS(
      'background-color',
      theme === 'dark' ? 'rgb(6, 17, 31)' : 'rgb(244, 248, 253)',
    );
    expect(await page.locator('body').innerText()).not.toContain('Checkout');
    await page.getByRole('button').focus();
    await expect(page.getByRole('button')).toBeFocused();
    await context.setOffline(false);
    await page.reload();
    await expect(page.locator('main h1')).toBeVisible();
  }
  await receipt('offline', {
    publicPaths: paths,
    keyboardRetry: true,
    privateContentAbsent: true,
    selectedThemeOverridesDevice: ['light', 'dark'],
  });
});

test('regular browser profile reports no installability errors', async ({ browserName }, info) => {
  expect(browserName).toBe('chromium');
  const context = await chromium.launchPersistentContext(
    info.outputPath('public-install-profile'),
    {
      headless: true,
      baseURL: process.env.SP_BROWSER_ORIGIN ?? 'http://127.0.0.1:58097',
      ...(process.env.SP_BROWSER_EXECUTABLE === undefined
        ? {}
        : { executablePath: process.env.SP_BROWSER_EXECUTABLE }),
    },
  );
  const page = context.pages()[0] ?? (await context.newPage());
  observeResponses(page);
  try {
    await page.goto('/overview');
    await expect(page.locator('main h1')).toBeVisible();
    const protocol = await context.newCDPSession(page);
    await protocol.send('Page.enable');
    await expect
      .poll(
        async () => (await protocol.send('Page.getInstallabilityErrors')).installabilityErrors,
        {
          timeout: 15_000,
        },
      )
      .toEqual([]);
    await protocol.detach();
    await receipt('installability', {
      errors: [],
      chromiumProtocolChecked: true,
      regularProfile: true,
      httpFailures: responseFailures.get(page) ?? [],
    });
  } catch (error) {
    await info.attach('public-install-http-statuses', {
      body: JSON.stringify(responseFailures.get(page) ?? []),
      contentType: 'application/json',
    });
    throw error;
  } finally {
    await context.close();
  }
});

test('bounded concurrent guest reads preserve valid page envelopes', async ({ page }) => {
  const queries = new Map<string, string>();
  const paths = [
    '/api/v1/resources',
    '/api/v1/topology',
    '/api/v1/metrics/current',
    '/api/v1/events',
  ];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (paths.includes(url.pathname)) queries.set(url.pathname, `${url.pathname}${url.search}`);
  });
  const setupRoutes = ['/overview', '/resources', '/topology', '/events'];
  for (const route of setupRoutes) {
    await page.goto(route);
    await expect(page.locator('main h1'), `${route} setup route`).toBeVisible();
    if (route === '/overview') await expect(page.locator('.source-card').first()).toBeVisible();
    if (route === '/resources')
      await expect(page.locator('.resource-list .resource-row')).toHaveCount(9);
    if (route === '/topology')
      await expect(page.locator('.topology-canvas .topology-node')).toHaveCount(9);
    if (route === '/events')
      await expect(page.locator('.event-list article').first()).toBeVisible();
    const path = route === '/overview' ? '/api/v1/metrics/current' : `/api/v1${route}`;
    await expect
      .poll(() => queries.has(path), { message: `${route} public read query` })
      .toBe(true);
    await refillDefaultPeerBucket();
  }
  await page.goto('/search');
  await expect(page.locator('.resource-list .resource-row')).toHaveCount(9);
  await page.goto('about:blank');
  await refillDefaultPeerBucket();
  const durations: number[] = [];
  const started = Date.now();
  for (let round = 0; round < 10; round++) {
    const roundStart = Date.now();
    await Promise.all(
      Array.from({ length: 8 }, async (_, index) => {
        const path = paths[index % paths.length];
        const query = path === undefined ? undefined : queries.get(path);
        if (query === undefined) throw new Error('Public read fixture query is unavailable.');
        const readStarted = Date.now();
        const response = await page.request.get(query, { timeout: 10_000 });
        expect(response.status()).toBe(200);
        const body = (await response.json()) as {
          availability?: unknown;
          completeness?: unknown;
          items?: unknown;
          page?: { has_more?: unknown };
        };
        expect(['available', 'stale'].includes(String(body.availability))).toBe(true);
        expect(['complete', 'partial'].includes(String(body.completeness))).toBe(true);
        expect(Array.isArray(body.items) && body.items.length <= 200).toBe(true);
        if (path !== '/api/v1/metrics/current')
          expect(typeof body.page?.has_more === 'boolean').toBe(true);
        durations.push(Date.now() - readStarted);
      }),
    );
    const delay = Math.max(0, 600 - (Date.now() - roundStart));
    await new Promise((done) => setTimeout(done, delay));
  }
  const sorted = [...durations].sort((left, right) => left - right);
  await receipt('concurrentReads', {
    scope: 'Disposable synthetic guest fixture; default read rate; local observations only.',
    requests: durations.length,
    parallel: 8,
    rounds: 10,
    minimumRoundSpacingMs: 600,
    setupRateBucketRefillMs: defaultPeerBucketRefillMs,
    setupNavigationRefills: setupRoutes.length,
    setupNavigationRefillMs: defaultPeerBucketRefillMs,
    setupExcludedFromReadTiming: true,
    initialRateBucketRefillMs: defaultPeerBucketRefillMs,
    maximumOfferedReadsPerSecond: 13.34,
    elapsedMs: Date.now() - started,
    medianMs: sorted[Math.floor(sorted.length / 2)],
    p95Ms: sorted[Math.ceil(sorted.length * 0.95) - 1],
    successfulResponses: 80,
    envelopesValidated: true,
  });
});

test('public worker update waits for acceptance and preserves private cache exclusion', async ({
  page,
}, info) => {
  await page.goto('/overview');
  await expect(page.locator('main h1')).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(async () => (await navigator.serviceWorker.getRegistration())?.active?.state),
    )
    .toBe('activated');
  await page.reload();
  await expect
    .poll(() => page.evaluate(() => navigator.serviceWorker.controller !== null))
    .toBe(true);
  const original = await readPublicWorker(page, info);
  const revised = original.replace(
    /(const shellCache = 'stackplore-public-shell-)[^']+(';)/u,
    '$1browser-update-fixture$2',
  );
  expect(revised !== original).toBe(true);
  const override = process.env.SP_BROWSER_WORKER_OVERRIDE;
  if (override === undefined) throw new Error('Owned public worker fixture is unavailable.');
  await writeFile(override, revised, { mode: 0o600 });
  let navigations = 0;
  page.on('framenavigated', (frame) => {
    if (frame === page.mainFrame()) navigations++;
  });
  await page.evaluate(async () => {
    await (await navigator.serviceWorker.getRegistration())?.update();
  });
  const accept = page.getByRole('button', { name: 'Reload to update', exact: true });
  await expect(accept).toBeVisible({ timeout: 15_000 });
  expect(navigations).toBe(0);
  await accept.click();
  try {
    await expect.poll(() => navigations).toBeGreaterThan(0);
  } catch (error) {
    await info.attach('public-worker-state', {
      body: JSON.stringify(
        await page.evaluate(async () => {
          const registration = await navigator.serviceWorker.getRegistration();
          return {
            active: registration?.active?.state ?? null,
            waiting: registration?.waiting?.state ?? null,
            installing: registration?.installing?.state ?? null,
            controller: navigator.serviceWorker.controller?.state ?? null,
            publicCaches: await caches.keys(),
          };
        }),
      ),
      contentType: 'application/json',
    });
    throw error;
  }
  await expect(page.locator('main h1')).toBeVisible();
  const cacheNames = await page.evaluate(async () => caches.keys());
  expect(cacheNames).toEqual(['stackplore-public-shell-browser-update-fixture']);
  const paths = await page.evaluate(async () => {
    const paths: string[] = [];
    for (const name of await caches.keys())
      for (const request of await (await caches.open(name)).keys())
        paths.push(new URL(request.url).pathname);
    return paths.sort();
  });
  expect(paths).toEqual([
    '/brand/stackplore-favicon-64.png',
    '/offline.css',
    '/offline.html',
    '/offline.js',
  ]);
  await receipt('update', { explicitAcceptance: true, publicPaths: paths });
  await unlink(override);
});
