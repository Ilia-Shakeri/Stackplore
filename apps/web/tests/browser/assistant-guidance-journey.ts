import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, type Page } from '@playwright/test';
import { parseAssistantRunResponse } from '@stackplore/api-client';
import { waitForLiveAssistant } from './assistant-live-state';

// Production routes and the real local processor are used; no request is intercepted.
export async function assistantGuidanceJourney(
  page: Page,
  locale: 'en' | 'fa',
  role: 'administrator' | 'viewer',
) {
  const metadata = {
    source_revision: process.env.GITHUB_SHA,
    locale,
    role,
    status: 'failed',
    phase: 'selection',
    snapshot_status: 0,
    consent_sent: false,
    document_current: false,
    not_facts: false,
    witness_current: false,
    cleared: false,
    coverage_valid: false,
    guide_supported: false,
    guide_section_exact: false,
    guide_text_exact: false,
    guide_citation_exact: false,
    status_reconnections: 0,
  };
  const words = Object.assign(
    {},
    JSON.parse(
      readFileSync(resolve(`src/features/assistant/messages.${locale}.json`), 'utf8'),
    ) as Record<string, string>,
    JSON.parse(
      readFileSync(resolve(`src/features/assistant/runbooks.messages.${locale}.json`), 'utf8'),
    ) as Record<string, string>,
  );
  const label = (key: string) => {
    const value = words[key];
    if (value === undefined) throw new Error('Guidance translation unavailable');
    return value;
  };
  const resourcePage = page.url();
  const guideTitle = locale === 'en' ? 'Local check guide' : 'راهنمای بررسی محلی';
  const guideBody =
    locale === 'en' ? 'Inspect current service health.' : 'سلامت سرویس را بررسی کنید.';
  try {
    await page.getByRole('button', { name: label('assistant.open'), exact: true }).click();
    const panel = page.locator('.assistant');
    await panel.getByRole('button', { name: label('assistant.guide.choose'), exact: true }).click();
    await panel
      .getByRole('textbox', { name: label('runbooks.query'), exact: true })
      .fill(locale === 'en' ? 'check' : 'بررسی');
    metadata.phase = 'search';
    await panel.getByRole('button', { name: label('runbooks.search'), exact: true }).click();
    metadata.phase = 'selected_document';
    await panel.getByRole('button', { name: guideTitle, exact: true }).click({ timeout: 15_000 });
    metadata.phase = 'consent';
    await panel
      .getByRole('checkbox', { name: label('assistant.guide.consent'), exact: true })
      .check();
    await panel
      .getByRole('button', { name: label('assistant.guide.include'), exact: true })
      .click();
    await panel
      .getByRole('textbox', { name: label('assistant.question'), exact: true })
      .fill(
        locale === 'en'
          ? 'Return only one next_checks section. Copy text exactly from guidance[0].excerpt. Set citations to [guidance[0].id]. No other sections.'
          : 'فقط یک بخش next_checks بده. در text فقط مقدار فیلد guidance[0].excerpt را عیناً کپی کن؛ نام فیلد یا توضیح اضافه نده. در citations مقدار guidance[0].id را فقط یک بار بگذار. بخش دیگری نده.',
      );
    const admission = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        new URL(response.url()).pathname === '/api/v1/assistant/runs',
    );
    const ownedRead = page.waitForRequest(
      (request) =>
        request.method() === 'GET' &&
        /^\/api\/v1\/assistant\/runs\/ar_[a-f0-9]{64}$/u.test(new URL(request.url()).pathname),
      { timeout: 110_000 },
    );
    metadata.phase = 'admission';
    await panel.getByRole('button', { name: label('assistant.send'), exact: true }).click();
    const response = await admission;
    if (response.status() !== 202) throw new Error('Guidance admission failed');
    const posted = response.request().postDataJSON() as {
      reviewed_document?: { consent?: unknown };
    };
    metadata.consent_sent = posted.reviewed_document?.consent === true;
    metadata.phase = 'completion';
    try {
      metadata.status_reconnections = await waitForLiveAssistant(page, label('assistant.listen'));
    } catch {
      throw new Error('Bounded guidance completion was not published');
    }
    const id = new URL((await ownedRead).url()).pathname.split('/').at(-1);
    if (id === undefined || !/^ar_[a-f0-9]{64}$/u.test(id))
      throw new Error('Guidance run identity missing');
    metadata.phase = 'snapshot';
    const current = await page.request.get(`/api/v1/assistant/runs/${id}`);
    metadata.snapshot_status = current.status();
    if (current.status() !== 200) throw new Error('Guidance snapshot unavailable');
    const ready = parseAssistantRunResponse(await current.json(), id);
    metadata.phase = 'parsed';
    if (ready.content_state !== 'ready') throw new Error('Guidance response not ready');
    const guides = ready.citations.filter((citation) => citation.kind === 'document');
    const guide = guides[0];
    metadata.document_current =
      guides.length === 1 &&
      guide?.document_reference?.corpus_id === 'browser_approved' &&
      guide.document_reference.version === 2 &&
      guide.document_reference.document_id === 'safe_check' &&
      guide.document_reference.locale === locale;
    metadata.not_facts = ready.answer.sections.every(
      (section) => section.kind !== 'facts' || !section.citations.includes(guide?.id ?? ''),
    );
    metadata.guide_section_exact =
      ready.answer.sections.length === 1 && ready.answer.sections[0].kind === 'next_checks';
    metadata.guide_text_exact = ready.answer.sections[0].text === guideBody;
    metadata.guide_citation_exact =
      ready.answer.sections[0].citations.length === 1 &&
      ready.answer.sections[0].citations[0] === guide?.id;
    metadata.guide_supported =
      metadata.guide_section_exact && metadata.guide_text_exact && metadata.guide_citation_exact;
    metadata.coverage_valid =
      ready.coverage?.length === 2 &&
      ready.coverage[0]?.tool === 'inventory' &&
      ready.coverage[1]?.tool === 'reviewed_document' &&
      ready.coverage.every((coverage) => coverage.records === 1);
    metadata.phase = 'witness';
    await panel
      .getByRole('button', { name: label('assistant.guide.witness'), exact: true })
      .click();
    const witness = panel.getByRole('article', { name: label('runbooks.document'), exact: true });
    await expect(witness).toBeVisible();
    await expect(witness.getByRole('heading')).toBeFocused();
    await page.keyboard.press('Tab');
    await expect(witness.getByText(guideBody, { exact: true })).toBeFocused();
    metadata.witness_current = await witness.evaluate(
      (element, body) => element.textContent.includes(body),
      guideBody,
    );
    await witness.getByRole('button', { name: label('runbooks.close'), exact: true }).click();
    await expect(
      panel.getByRole('textbox', { name: label('assistant.question'), exact: true }),
    ).toBeFocused();
    await panel
      .getByRole('button', { name: label('assistant.guide.witness'), exact: true })
      .click();
    await expect(witness).toBeVisible();
    metadata.phase = 'clearing';
    await page.evaluate(() => {
      window.dispatchEvent(new Event('pagehide'));
    });
    metadata.cleared = await page.evaluate(
      (body) => !document.body.textContent.includes(body),
      guideBody,
    );
    metadata.phase = 'assertions';
    for (const key of [
      'consent_sent',
      'document_current',
      'not_facts',
      'coverage_valid',
      'guide_supported',
      'witness_current',
      'cleared',
    ] as const)
      expect(metadata[key]).toBe(true);
    metadata.status = 'passed';
    metadata.phase = 'complete';
  } finally {
    writeFileSync(
      resolve(`../../.tmp/hosted-guidance-${locale}-${role}.json`),
      `${JSON.stringify(metadata)}\n`,
      { mode: 0o600 },
    );
    await page.goto(resourcePage);
  }
}
