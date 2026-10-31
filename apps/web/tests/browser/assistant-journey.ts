import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test, type Page, type Route, type Response, type Request } from '@playwright/test';
import axe from 'axe-core';

// Real browser/client; closed synthetic private HTTP replies. No processor starts.
export async function assistantJourney(
  page: Page,
  locale: 'en' | 'fa',
  role: 'administrator' | 'viewer',
  healthHistoryLabel: string,
) {
  const text = JSON.parse(
    readFileSync(resolve(`src/features/assistant/messages.${locale}.json`), 'utf8'),
  ) as Record<string, string>;
  const label = (key: string) => {
    const value = text[key];
    if (value === undefined) throw new Error('Synthetic browser translation unavailable');
    return value;
  };
  const parts = new URL(page.url()).pathname.split('/').map((value) => decodeURIComponent(value));
  const source = parts[2],
    resource = parts[5];
  if (source === undefined || resource === undefined || !resource.startsWith('r1_'))
    throw new Error('Selected browser resource unavailable');
  const at = new Date();
  const time = at.toISOString();
  const range = { from: new Date(at.getTime() - 15 * 60_000).toISOString(), to: time };
  const runId = `ar_${'a'.repeat(64)}`,
    citationId = `ev_${'b'.repeat(64)}`;
  const run = {
    id: runId,
    conversation_id: `ac_${'c'.repeat(64)}`,
    goal: 'explain_resource',
    state: 'queued',
    version: 1,
    scope: { source_id: source, resource_id: resource, window_minutes: 15, range },
    created_at: time,
    updated_at: time,
    deadline: new Date(at.getTime() + 120_000).toISOString(),
    usage_uncertain: false,
    usage_conservative: false,
  };
  const complete = {
    ...run,
    state: 'completed',
    version: 3,
    updated_at: new Date(at.getTime() + 1).toISOString(),
  };
  const citation = {
    id: citationId,
    kind: 'resource',
    source_id: source,
    resource_id: resource,
    range,
    observed_at: time,
  };
  const question =
    locale === 'fa' ? 'وضعیت این منبع آزمایشی را توضیح بده.' : 'Explain this synthetic resource.';
  const answer =
    locale === 'fa'
      ? 'شواهد این منبع آزمایشی در دسترس است.'
      : 'Evidence for this synthetic resource is available.';
  const counts = { admit: 0, status: 0, read: 0, citation: 0 };
  let historyPhase = 'not_started';
  let privateRequests = 0;
  const observeRequest = (request: Request) => {
    if (new URL(request.url()).pathname.startsWith('/api/v1/assistant/')) privateRequests += 1;
  };
  const failedStatuses: number[] = [];
  const observe = (response: Response) => {
    if (response.status() >= 400 && failedStatuses.length < 16)
      failedStatuses.push(response.status());
  };
  const pattern = '**/api/v1/assistant/**';
  const handler = async (route: Route) => {
    const request = route.request(),
      url = new URL(request.url());
    const headers = {
      'Cache-Control': 'no-store',
      Vary: 'Cookie',
      'X-Request-ID': request.headers()['x-request-id'] ?? 'req_synthetic_browser',
    };
    if (url.search !== '') throw new Error('Synthetic private route received a query');
    if (request.method() === 'POST' && url.pathname === '/api/v1/assistant/runs') {
      counts.admit += 1;
      const input = request.postDataJSON() as Record<string, unknown>;
      if (
        input.source_id !== source ||
        input.resource_id !== resource ||
        input.window_minutes !== 15 ||
        input.locale !== locale ||
        input.question !== question
      )
        throw new Error('Synthetic request scope mismatch');
      expect(/^[A-Za-z0-9_-]{43}$/u.test(request.headers()['x-stackplore-csrf'] ?? '')).toBe(true);
      expect(/^[0-9a-f]{64}$/u.test(request.headers()['idempotency-key'] ?? '')).toBe(true);
      await route.fulfill({
        status: 202,
        headers,
        contentType: 'application/json',
        body: JSON.stringify({ run }),
      });
    } else if (
      request.method() === 'GET' &&
      url.pathname === `/api/v1/assistant/runs/${runId}/events`
    ) {
      counts.status += 1;
      const event = {
        run_id: runId,
        sequence: 3,
        version: 3,
        state: 'completed',
        at: complete.updated_at,
      };
      await route.fulfill({
        status: 200,
        headers,
        contentType: 'text/event-stream; charset=utf-8',
        body: `id: 3\nevent: assistant.status.v1\ndata: ${JSON.stringify(event)}\n\n`,
      });
    } else if (request.method() === 'GET' && url.pathname === `/api/v1/assistant/runs/${runId}`) {
      counts.read += 1;
      await route.fulfill({
        status: 200,
        headers,
        contentType: 'application/json',
        body: JSON.stringify({
          run: complete,
          content_state: 'ready',
          answer: { sections: [{ kind: 'facts', text: answer, citations: [citationId] }] },
          citations: [citation],
        }),
      });
    } else if (
      request.method() === 'GET' &&
      url.pathname === `/api/v1/assistant/runs/${runId}/citations/${citationId}`
    ) {
      counts.citation += 1;
      await route.fulfill({
        status: 200,
        headers,
        contentType: 'application/json',
        body: JSON.stringify(citation),
      });
    } else throw new Error('Unexpected synthetic private route');
  };
  const viewport = page.viewportSize();
  await page.route(pattern, handler);
  page.on('response', observe);
  page.on('request', observeRequest);
  try {
    await page.setViewportSize({ width: 390, height: 844 });
    const historyTable = page.getByRole('table', { name: healthHistoryLabel, exact: true });
    await expect(historyTable).toBeVisible();
    expect(
      await historyTable.locator('th, td').evaluateAll((cells) =>
        cells.every((cell) => {
          const style = getComputedStyle(cell);
          return (
            Number.parseFloat(style.paddingInlineStart) >= 4 &&
            Number.parseFloat(style.paddingInlineEnd) >= 4
          );
        }),
      ),
    ).toBe(true);
    const open = page.getByRole('button', { name: label('assistant.open'), exact: true });
    await open.focus();
    await page.keyboard.press('Enter');
    const panel = page.locator('.assistant');
    const input = panel.getByRole('textbox', { name: label('assistant.question'), exact: true });
    await expect(input).toBeVisible();
    expect(counts).toEqual({ admit: 0, status: 0, read: 0, citation: 0 });
    await input.fill(question);
    await input.press('Tab');
    await expect(
      panel.getByRole('button', { name: label('assistant.send'), exact: true }),
    ).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(panel.getByText(answer, { exact: true })).toBeVisible();
    expect(counts).toEqual({ admit: 1, status: 1, read: 1, citation: 0 });
    await panel.getByRole('button', { name: label('assistant.citation'), exact: true }).click();
    await expect(page.locator('#resource-detail-name')).toBeFocused();
    expect(
      await page.locator('#resource-detail-name').evaluate((element) => {
        const rect = element.getBoundingClientRect();
        const node = document.elementFromPoint(
          rect.left + rect.width / 2,
          rect.top + rect.height / 2,
        );
        return node !== null && (node === element || element.contains(node));
      }),
    ).toBe(true);
    expect(counts.citation).toBe(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(
      true,
    );
    expect(decodeURIComponent(page.url()).includes(question)).toBe(false);
    const stored = await page.evaluate(() =>
      [localStorage, sessionStorage].flatMap((storage) =>
        Array.from({ length: storage.length }, (_value, index) =>
          storage.getItem(storage.key(index) ?? ''),
        ),
      ),
    );
    expect(stored.some((value) => value?.includes(question))).toBe(false);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    for (const theme of ['light', 'dark']) {
      await page.evaluate((value) => {
        document.documentElement.dataset.theme = value;
      }, theme);
      await page.evaluate(axe.source);
      const violations = await panel.evaluate(async (element) => {
        const result = await window.axe.run(element, {
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
      expect(violations).toEqual([]);
      await panel
        .getByRole('heading', { name: label('assistant.title'), exact: true })
        .scrollIntoViewIfNeeded();
      await page.screenshot({
        fullPage: false,
        path: test.info().outputPath(`assistant-${locale}-${role}-${theme}-mobile.png`),
      });
    }
    const resourceUrl = page.url();
    const routeText = JSON.parse(
      readFileSync(resolve(`src/routes/messages.${locale}.json`), 'utf8'),
    ) as Record<string, string>;
    const aboutTitle = routeText['route.about.title'];
    if (aboutTitle === undefined) throw new Error('About readiness fixture unavailable');
    // Routing disables Chromium's HTTP cache. Restore normal asset caching for
    // document-history checks, while still observing every private request.
    await page.unroute(pattern, handler);
    for (const settled of [true, false]) {
      historyPhase = settled ? 'settled_destination' : 'immediate_back';
      // Same minimum peer-bucket refill cadence as the existing release gate.
      await new Promise<void>((done) => {
        setTimeout(done, 2500);
      });
      await page.locator('.assistant').getByRole('textbox').fill(question);
      await page.goto('/about');
      if (settled)
        await expect(page.getByRole('heading', { name: aboutTitle, exact: true })).toBeVisible();
      await page.goBack();
      await expect(page).toHaveURL(resourceUrl);
      await expect(page.locator('#resource-detail-name')).toBeVisible();
      const reopen = page.getByRole('button', { name: label('assistant.open'), exact: true });
      if (await reopen.isVisible()) await reopen.click();
      await expect(page.locator('.assistant').getByRole('textbox')).toHaveValue('');
      await expect(page.locator('.assistant').getByText(question, { exact: true })).toHaveCount(0);
      expect(counts.admit).toBe(1);
      expect(privateRequests).toBe(4);
    }
    await page.getByRole('button', { name: label('assistant.close'), exact: true }).click();
    await expect(page.locator('.assistant textarea')).toHaveCount(0);
  } catch (error) {
    const documentState = await page
      .evaluate(() => ({
        pathClass: location.pathname.startsWith('/resources/')
          ? 'resource'
          : location.pathname === '/about'
            ? 'about'
            : location.pathname === '/login'
              ? 'login'
              : 'other',
        ready: document.readyState,
        main: document.querySelector('main') !== null,
        heading: document.getElementById('resource-detail-name') !== null,
        bodyCharacters: document.body.textContent.length,
      }))
      .catch(() => ({ pathClass: 'unavailable' }));
    process.stderr.write(
      JSON.stringify({
        component: 'synthetic-conversation-history',
        locale,
        role,
        counts,
        privateRequests,
        historyPhase,
        failedStatuses,
        documentState,
      }) + '\n',
    );
    throw error;
  } finally {
    page.off('response', observe);
    page.off('request', observeRequest);
    await page.unroute(pattern, handler);
    await page.emulateMedia({ reducedMotion: 'no-preference' });
    await page
      .evaluate(() => {
        document.documentElement.dataset.theme = 'light';
      })
      .catch(() => undefined);
    if (viewport !== null) await page.setViewportSize(viewport);
  }
}
