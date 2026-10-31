import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { expect, test, type Locator, type Page } from '@playwright/test';
import { assistantJourney } from './assistant-journey';

const locale = process.env.SP_BROWSER_LOCALE === 'fa' ? 'fa' : 'en';
const text = Object.assign(
  {},
  ...[
    'auth/messages',
    'users/messages',
    'product/trend.messages',
    'product/exploration.messages',
  ].map(
    (path) =>
      JSON.parse(readFileSync(resolve(`src/features/${path}.${locale}.json`), 'utf8')) as Record<
        string,
        string
      >,
  ),
) as Record<string, string>;
const label = (key: string) => {
  const value = text[key];
  if (value === undefined) throw new Error('Browser fixture translation is unavailable.');
  return value;
};

async function privateInput(locator: Locator, value: string) {
  try {
    await locator.evaluate((element, privateValue) => {
      if (!(element instanceof HTMLInputElement)) throw new Error('Private form unavailable.');
      element.value = privateValue;
      element.dispatchEvent(new Event('input', { bubbles: true }));
      element.dispatchEvent(new Event('change', { bubbles: true }));
    }, value);
  } catch {
    throw new Error('Private browser fixture form could not be filled.');
  }
}

async function logout(page: Page) {
  await page.getByRole('button', { name: label('auth.userMenu.open') }).click();
  await page.getByRole('menuitem', { name: label('auth.logout.action'), exact: true }).click();
  await expect(page).toHaveURL(/\/login/u);
  expect((await page.request.get('/api/v1/auth/session')).status()).toBe(401);
}

test(`normal ${locale}: setup, recent authentication, role boundaries, history and logout`, async ({
  page,
  context,
}) => {
  const proof = process.env.SP_BROWSER_BOOTSTRAP_PROOF ?? '';
  if (proof.length !== 43) throw new Error('Private bootstrap fixture is unavailable.');
  const administratorPassword = randomBytes(24).toString('base64url');
  const viewerPassword = randomBytes(24).toString('base64url');
  const rejectedPassword = randomBytes(24).toString('base64url');
  const responses: { path: string; status: number }[] = [];
  page.on('response', (response) => {
    const path = new URL(response.url()).pathname;
    if (['/api/v1/setup', '/api/v1/auth/recent-grants', '/api/v1/users'].includes(path)) {
      responses.push({ path, status: response.status() });
    }
  });
  await page.addInitScript((locale) => {
    localStorage.setItem('stackplore.locale', locale);
    localStorage.setItem('stackplore.theme', 'light');
  }, locale);
  try {
    await page.goto('/overview');
    await expect(page).toHaveURL(/\/setup/u);
    await page.locator('input[name="display_name"]').fill('Browser Administrator');
    await page.locator('input[name="login_name"]').fill('browser_admin');
    await privateInput(page.locator('input[name="password"]'), administratorPassword);
    await privateInput(page.locator('input[name="password_confirmation"]'), administratorPassword);
    await privateInput(page.locator('input[name="bootstrap_proof"]'), proof);
    await page.getByRole('button', { name: label('auth.setup.submit'), exact: true }).click();
    await expect(page).toHaveURL(/\/overview/u);
    expect(responses.some(({ path, status }) => path === '/api/v1/setup' && status === 201)).toBe(
      true,
    );
    const setupStatus = (await (await page.request.get('/api/v1/setup/status')).json()) as {
      state?: unknown;
    };
    expect(setupStatus.state === 'closed').toBe(true);
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'fa' ? 'rtl' : 'ltr');

    await page.goto('/settings/users');
    await page.getByRole('button', { name: label('users.page.add'), exact: true }).click();
    const dialog = page.getByRole('dialog');
    await dialog.locator('input[name="display_name"]').fill('Browser Viewer');
    await dialog.locator('input[name="login_name"]').fill('browser_viewer');
    await dialog.locator('select[name="role"]').selectOption('viewer');
    await privateInput(dialog.locator('input[name="password"]'), viewerPassword);
    await privateInput(dialog.locator('input[name="password_confirmation"]'), viewerPassword);
    await privateInput(dialog.locator('input[name="current_password"]'), rejectedPassword);
    await dialog.getByRole('button', { name: label('users.create.submit'), exact: true }).click();
    await expect(dialog.getByRole('alert')).toBeVisible();
    expect(
      responses.some(({ path, status }) => path === '/api/v1/auth/recent-grants' && status === 403),
    ).toBe(true);
    expect(responses.some(({ path, status }) => path === '/api/v1/users' && status === 201)).toBe(
      false,
    );

    await privateInput(dialog.locator('input[name="password"]'), viewerPassword);
    await privateInput(dialog.locator('input[name="password_confirmation"]'), viewerPassword);
    await privateInput(dialog.locator('input[name="current_password"]'), administratorPassword);
    await dialog.getByRole('button', { name: label('users.create.submit'), exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText('Browser Viewer', { exact: true })).toBeVisible();
    expect(
      responses.some(({ path, status }) => path === '/api/v1/auth/recent-grants' && status === 201),
    ).toBe(true);
    expect(responses.some(({ path, status }) => path === '/api/v1/users' && status === 201)).toBe(
      true,
    );

    await page.goto('/audit');
    await expect(page.locator('.audit-list article').first()).toBeVisible();
    expect((await page.request.get('/api/v1/audit')).status()).toBe(200);
    await page.goto('/settings');
    const settings = (await (await page.request.get('/api/v1/settings/application')).json()) as {
      mutation_mode?: {
        enabled?: unknown;
        docker_write_enabled?: unknown;
        kubernetes_write_enabled?: unknown;
      };
    };
    expect(
      settings.mutation_mode?.enabled === false &&
        settings.mutation_mode.docker_write_enabled === false &&
        settings.mutation_mode.kubernetes_write_enabled === false,
    ).toBe(true);

    await page.goto('/resources');
    await page.locator('main a[href^="/resources/"]').first().click();
    await expect(
      page.getByRole('heading', { name: label('insight.resourceHistory'), exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('table', { name: label('insight.healthHistory'), exact: true }),
    ).toBeVisible();
    await expect(
      page
        .getByRole('table', { name: label('insight.healthHistory'), exact: true })
        .locator('tbody tr')
        .first(),
    ).toBeVisible();
    await assistantJourney(page, locale, 'administrator', label('insight.healthHistory'));
    await page.locator('main a[href^="/logs?"]').click();
    const pendingDownload = page.waitForEvent('download');
    await page.getByRole('button', { name: label('explore.logs.download'), exact: true }).click();
    const download = await pendingDownload;
    expect(download.suggestedFilename()).toBe('stackplore-logs.txt');
    const stream = await download.createReadStream();
    const chunks: Buffer[] = [];
    let bytes = 0;
    for await (const chunk of stream) {
      const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk as Uint8Array);
      bytes += buffer.length;
      if (bytes > 1048576) throw new Error('Browser download exceeded its public byte limit.');
      chunks.push(buffer);
    }
    expect(Buffer.concat(chunks).toString('utf8').endsWith('# stackplore-export: complete\n')).toBe(
      true,
    );
    await logout(page);

    await page.locator('input[name="login_name"]').fill('browser_viewer');
    await privateInput(page.locator('input[name="password"]'), viewerPassword);
    await page.getByRole('button', { name: label('auth.login.submit'), exact: true }).click();
    await expect(page).toHaveURL(/\/overview/u);
    expect((await page.request.get('/api/v1/users')).status()).toBe(403);
    expect((await page.request.get('/api/v1/audit')).status()).toBe(403);
    await page.goto('/settings/users');
    await expect(page).toHaveURL(/\/overview/u);
    await expect(
      page.getByRole('button', { name: label('users.page.add'), exact: true }),
    ).toHaveCount(0);
    await page.goto('/resources');
    await page.locator('main a[href^="/resources/"]').first().click();
    await assistantJourney(page, locale, 'viewer', label('insight.healthHistory'));
    const cachePaths = await page.evaluate(async () => {
      const paths: string[] = [];
      for (const name of await caches.keys())
        for (const request of await (await caches.open(name)).keys())
          paths.push(new URL(request.url).pathname);
      return paths.sort();
    });
    expect(cachePaths).toEqual([
      '/brand/stackplore-favicon-64.png',
      '/offline.css',
      '/offline.html',
      '/offline.js',
    ]);
    await logout(page);
  } finally {
    await page
      .evaluate(() => {
        for (const input of document.querySelectorAll<HTMLInputElement>('input[type="password"]'))
          input.value = '';
      })
      .catch(() => undefined);
    await context.clearCookies();
    await page.close();
  }
});
