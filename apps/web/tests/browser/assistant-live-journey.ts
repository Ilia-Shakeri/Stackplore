import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, type Page, type Request, type Response } from '@playwright/test';

// No routes are intercepted. Replies pass through the production application.
export async function assistantLiveJourney(
  page: Page,
  locale: 'en' | 'fa',
  role: 'administrator' | 'viewer',
) {
  const metadata = {
    source_revision: process.env.GITHUB_SHA,
    locale,
    role,
    admissions: 0,
    admission_status: 0,
    run_state: 'not_observed',
    content_state: 'not_observed',
    resource_identity_supported: false,
    citation_current: false,
    cleared: false,
    view_state: 'not_visible',
  };
  const reads: Promise<void>[] = [];
  const observeReply = (response: Response) => {
    const path = new URL(response.url()).pathname;
    if (response.request().method() === 'POST' && path === '/api/v1/assistant/runs') {
      metadata.admission_status = response.status();
      return;
    }
    if (
      reads.length >= 8 ||
      response.request().method() !== 'GET' ||
      !/^\/api\/v1\/assistant\/runs\/ar_[a-f0-9]{64}$/u.test(path)
    ) {
      return;
    }
    reads.push(
      response
        .json()
        .then((value: unknown) => {
          if (typeof value !== 'object' || value === null) return;
          const body = value as Record<string, unknown>;
          if (typeof body.run === 'object' && body.run !== null) {
            const state = (body.run as Record<string, unknown>).state;
            if (
              typeof state === 'string' &&
              [
                'queued',
                'running',
                'completed',
                'failed',
                'cancelled',
                'expired',
                'interrupted',
                'uncertain',
              ].includes(state)
            ) {
              metadata.run_state = state;
            }
          }
          if (
            typeof body.content_state === 'string' &&
            ['pending', 'ready', 'volatile_lost', 'unavailable'].includes(body.content_state)
          ) {
            metadata.content_state = body.content_state;
          }
        })
        .catch(() => {
          metadata.content_state = 'unreadable';
        }),
    );
  };
  const words = JSON.parse(
    readFileSync(resolve(`src/features/assistant/messages.${locale}.json`), 'utf8'),
  ) as Record<string, string>;
  const label = (key: string) => {
    const value = words[key];
    if (value === undefined) throw new Error('Real browser translation unavailable');
    return value;
  };
  const original = page.viewportSize();
  await page.setViewportSize({ width: 1536, height: 960 });
  const heading = page.locator('#resource-detail-name');
  const identity = new URL(page.url()).pathname.split('/').map(decodeURIComponent)[5];
  if (identity === undefined || !identity.startsWith('r1_'))
    throw new Error('Selected resource is unavailable');
  let admissions = 0;
  const observe = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (request.method() === 'POST' && path === '/api/v1/assistant/runs') {
      admissions++;
    }
  };
  page.on('request', observe);
  page.on('response', observeReply);
  try {
    await page.getByRole('button', { name: label('assistant.open'), exact: true }).click();
    const panel = page.locator('.assistant');
    const input = panel.getByRole('textbox', { name: label('assistant.question'), exact: true });
    const question =
      locale === 'fa'
        ? 'فقط یک بخش facts بده. متن آن دقیقاً resource_id در شاهد باشد؛ همان شناسه شاهد را یک بار ارجاع بده. هیچ ادعای سلامت یا علت یا متن دیگر نده.'
        : 'Return exactly one facts section. Its text must be exactly the resource_id in the evidence. Cite its evidence id once. No health, cause or other claims.';
    expect(admissions).toBe(0);
    await input.fill(question);
    await input.press('Tab');
    await expect(
      panel.getByRole('button', { name: label('assistant.send'), exact: true }),
    ).toBeFocused();
    await page.keyboard.press('Enter');
    // Avoid locator-error DOM dumps containing a generated reply.
    try {
      await page.waitForFunction(
        () => document.querySelectorAll('.assistant__turns .assistant__text').length > 0,
        undefined,
        { timeout: 110_000 },
      );
    } catch {
      throw new Error('Real bounded resource completion was not published');
    }
    const supported = await panel.evaluate((element, expected) => {
      const texts = element.querySelectorAll('.assistant__turns .assistant__text');
      return texts.length === 1 && texts[0]?.textContent.trim() === expected;
    }, identity);
    expect(supported).toBe(true);
    metadata.resource_identity_supported = supported;
    const citation = panel.getByRole('button', { name: label('assistant.citation'), exact: true });
    expect(await citation.count()).toBe(1);
    await citation.click();
    metadata.citation_current = await heading.evaluate(
      (element) => document.activeElement === element,
    );
    expect(metadata.citation_current).toBe(true);
    expect(admissions).toBe(1);
    expect(new URL(page.url()).search.includes(question)).toBe(false);
    const stored = await page.evaluate((privateQuestion) => {
      return [localStorage, sessionStorage].some((storage) =>
        Array.from({ length: storage.length }, (_value, index) =>
          storage.getItem(storage.key(index) ?? ''),
        ).some((value) => value?.includes(privateQuestion)),
      );
    }, question);
    expect(stored).toBe(false);
    await panel.getByRole('button', { name: label('assistant.close'), exact: true }).click();
    expect(await page.locator('.assistant__text').count()).toBe(0);
    await page.getByRole('button', { name: label('assistant.open'), exact: true }).click();
    expect(await page.locator('.assistant textarea').inputValue()).toBe('');
    metadata.cleared = true;
    expect(admissions).toBe(1);
    await page.getByRole('button', { name: label('assistant.close'), exact: true }).click();
  } finally {
    page.off('request', observe);
    page.off('response', observeReply);
    await Promise.allSettled(reads);
    metadata.admissions = admissions;
    try {
      const states = await page.locator('.assistant [role="status"]').allTextContents();
      metadata.view_state =
        Object.entries(words).find(
          ([key, value]) => key.startsWith('assistant.state.') && states.includes(value),
        )?.[0] ?? 'not_visible';
    } catch {
      metadata.view_state = 'not_visible';
    }
    writeFileSync(
      resolve(`../../.tmp/hosted-live-${locale}-${role}.json`),
      `${JSON.stringify(metadata)}\n`,
      { mode: 0o600 },
    );
    if (original !== null) await page.setViewportSize(original);
  }
}
