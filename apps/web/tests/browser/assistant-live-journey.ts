import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, type Page, type Request } from '@playwright/test';

// No routes are intercepted. Replies pass through the production application.
export async function assistantLiveJourney(page: Page, locale: 'en' | 'fa') {
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
  const name = (await heading.textContent())?.trim();
  if (name === undefined || name === '') throw new Error('Selected resource is unavailable');
  let admissions = 0;
  const observe = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (request.method() === 'POST' && path === '/api/v1/assistant/runs') {
      admissions++;
    }
  };
  page.on('request', observe);
  try {
    await page.getByRole('button', { name: label('assistant.open'), exact: true }).click();
    const panel = page.locator('.assistant');
    const input = panel.getByRole('textbox', { name: label('assistant.question'), exact: true });
    const question =
      locale === 'fa'
        ? 'فقط یک بخش facts بده. متن آن دقیقاً نام منبع در شاهد باشد؛ همان شناسه شاهد را یک بار ارجاع بده. هیچ ادعای سلامت یا علت یا متن دیگر نده.'
        : 'Return exactly one facts section. Its text must be exactly the resource name in the evidence. Cite its evidence id once. No health, cause or other claims.';
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
    const supported = await panel.evaluate(
      (element, expected) => {
        const texts = element.querySelectorAll('.assistant__turns .assistant__text');
        return texts.length === 1 && texts[0]?.textContent?.trim() === expected;
      },
      name,
    );
    expect(supported).toBe(true);
    const citation = panel.getByRole('button', { name: label('assistant.citation'), exact: true });
    expect(await citation.count()).toBe(1);
    await citation.click();
    expect(await heading.evaluate((element) => document.activeElement === element)).toBe(true);
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
    expect(admissions).toBe(1);
    await page.getByRole('button', { name: label('assistant.close'), exact: true }).click();
  } finally {
    page.off('request', observe);
    if (original !== null) await page.setViewportSize(original);
  }
}
