import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, type Page, type Request, type Response } from '@playwright/test';
import { parseAssistantRunResponse } from '@stackplore/api-client';
import { waitForLiveAssistant } from './assistant-live-state';
import { measureReads, prepareWorkload, workloadReceipt } from './hosted-workload';

// No routes are intercepted. Replies pass through the production application.
export async function assistantLiveJourney(
  page: Page,
  locale: 'en' | 'fa',
  role: 'administrator' | 'viewer',
  goal: 'explain_resource' | 'investigate_incident' = 'explain_resource',
) {
  const loadQueries = process.env.SP_STAGE9_WORKLOAD === 'true' && locale === 'en' && role === 'administrator' && goal === 'investigate_incident'
    ? await prepareWorkload(page) : undefined;
  const metadata = {
    source_revision: process.env.GITHUB_SHA,
    locale,
    role,
    goal,
    coverage_valid: goal === 'explain_resource',
    citation_count: 0,
    admissions: 0,
    admission_status: 0,
    run_state: 'not_observed',
    content_state: 'not_observed',
    resource_identity_supported: false,
    identity_contained: false,
    identity_text_bytes: 0,
    identity_shape: 'unobserved',
    citation_current: false,
    citation_status: 0,
    report_current: goal === 'explain_resource',
    report_cleared: goal === 'explain_resource',
    report_rows: 0,
    report_status: 0,
    cleared: false,
    view_state: 'not_visible',
    status_reconnections: 0,
  };
  let ownedRunID: string | undefined;
  const observeReply = (response: Response) => {
    const path = new URL(response.url()).pathname;
    if (
      response.request().method() === 'GET' &&
      /^\/api\/v1\/assistant\/runs\/ar_[a-f0-9]{64}\/report$/u.test(path)
    )
      metadata.report_status = response.status();
    if (
      response.request().method() === 'GET' &&
      /^\/api\/v1\/assistant\/runs\/ar_[a-f0-9]{64}\/citations\/ev_[a-f0-9]{64}$/u.test(path)
    )
      metadata.citation_status = response.status();
    if (response.request().method() === 'POST' && path === '/api/v1/assistant/runs')
      metadata.admission_status = response.status();
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
  const resourcePage = page.url();
  await page.setViewportSize({ width: 1536, height: 960 });
  const heading = page.locator('#resource-detail-name');
  const identity = new URL(page.url()).pathname.split('/').map(decodeURIComponent)[5];
  if (identity === undefined || !identity.startsWith('r1_'))
    throw new Error('Selected resource is unavailable');
  let admissions = 0;
  const observe = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (
      admissions === 1 &&
      request.method() === 'GET' &&
      /^\/api\/v1\/assistant\/runs\/ar_[a-f0-9]{64}$/u.test(path)
    )
      ownedRunID ??= path.split('/').at(-1);
    if (request.method() === 'POST' && path === '/api/v1/assistant/runs') {
      admissions++;
    }
  };
  page.on('request', observe);
  page.on('response', observeReply);
  try {
    await page.getByRole('button', { name: label('assistant.open'), exact: true }).click();
    const panel = page.locator('.assistant');
    if (goal === 'investigate_incident') {
      await panel
        .getByRole('combobox', { name: label('assistant.mode'), exact: true })
        .selectOption(goal);
      expect(await panel.getByRole('checkbox').isChecked()).toBe(false);
    }
    const input = panel.getByRole('textbox', { name: label('assistant.question'), exact: true });
    const question =
      locale === 'fa'
        ? 'فقط یک بخش facts بده. text را عیناً از evidence[0].text.records[0].resource_id کپی کن. citations فقط [evidence[0].id]. هیچ بخش دیگری نده.'
        : 'Return only one facts section. Copy text exactly from evidence[0].text.records[0].resource_id. Set citations to [evidence[0].id]. No other sections.';
    expect(admissions).toBe(0);
    await input.fill(question);
    const idleReads = loadQueries === undefined ? undefined : await measureReads(page, loadQueries);
    await input.press('Tab');
    await expect(
      panel.getByRole('button', { name: label('assistant.send'), exact: true }),
    ).toBeFocused();
    await page.keyboard.press('Enter');
    // Avoid locator-error DOM dumps containing a generated reply.
    try {
      const [reconnections, busyReads] = await Promise.all([
        waitForLiveAssistant(page, label('assistant.listen')),
        loadQueries === undefined ? undefined : measureReads(page, loadQueries),
      ]);
      metadata.status_reconnections = reconnections;
      if (idleReads !== undefined && busyReads !== undefined) {
        workloadReceipt('load', {
          status: 'collected', scope: 'Same-source enabled profile: idle versus one actual admitted generation; not feature-off/on baseline',
          idle: idleReads, busy: busyReads, maximum_p95_ratio: 1.1,
          p95_ratio: busyReads.latency_p95_ms / Math.max(1, idleReads.latency_p95_ms),
          release_accepted: false,
        });
      }
    } catch {
      throw new Error('Real bounded resource completion was not published');
    }
    // The UI may abort its completed fetch while DevTools reads the same body.
    // Read the current owning snapshot separately, before navigating or clearing.
    if (ownedRunID === undefined || !/^ar_[a-f0-9]{64}$/u.test(ownedRunID))
      throw new Error('Real owning run identity was not observed');
    try {
      const response = await page.request.get(`/api/v1/assistant/runs/${ownedRunID}`, {
        timeout: 15_000,
      });
      if (response.status() !== 200 || response.headers()['cache-control'] !== 'no-store')
        throw new Error('snapshot_unavailable');
      const body = parseAssistantRunResponse(await response.json(), ownedRunID);
      metadata.run_state = body.run.state;
      metadata.content_state = body.content_state;
      if (body.content_state !== 'ready') throw new Error('snapshot_not_ready');
      metadata.citation_count = body.citations.length;
      if (goal === 'investigate_incident') {
        const expected = [
          'inventory',
          'health',
          'metrics',
          'events',
          'topology',
          'alert_history',
          'workload_history',
        ];
        const coverage = body.coverage;
        metadata.coverage_valid =
          coverage !== undefined &&
          coverage.length === expected.length &&
          coverage.every((item, index) => item.tool === expected[index]);
      }
    } catch {
      throw new Error('Real current owning snapshot was not verified');
    }
    expect(metadata.run_state).toBe('completed');
    expect(metadata.coverage_valid).toBe(true);
    const identityProbe = await panel.evaluate((element, expected) => {
      const texts = element.querySelectorAll('.assistant__turns .assistant__text');
      const value = texts[0]?.textContent.trim() ?? '';
      return {
        supported: texts.length === 1 && value === expected,
        contained: value.includes(expected),
        text_bytes: new TextEncoder().encode(value).byteLength,
        shape: /^r1_[A-Za-z0-9_-]{43}$/u.test(value)
          ? 'resource'
          : /^ev_[a-f0-9]{64}$/u.test(value)
            ? 'citation'
            : /^e[1-9][0-9]?$/u.test(value)
              ? 'private_handle'
              : 'other',
      };
    }, identity);
    const supported = identityProbe.supported;
    metadata.identity_contained = identityProbe.contained;
    metadata.identity_text_bytes = identityProbe.text_bytes;
    metadata.identity_shape = identityProbe.shape;
    metadata.resource_identity_supported = supported;
    expect(supported).toBe(true);
    const citation = panel.getByRole('button', { name: label('assistant.citation'), exact: true });
    const count = await citation.count();
    if (goal === 'explain_resource') expect(count).toBe(1);
    else {
      expect(count).toBeGreaterThanOrEqual(1);
      expect(count).toBeLessThanOrEqual(64);
      expect(
        await panel
          .getByRole('heading', { name: label('assistant.coverage'), exact: true })
          .count(),
      ).toBe(1);
    }
    await citation.last().click();
    try {
      await page.waitForFunction(
        () => document.activeElement?.id === 'resource-detail-name',
        undefined,
        { timeout: 5000 },
      );
    } catch {
      throw new Error('Real citation authority focus did not settle');
    }
    metadata.citation_current = await heading.evaluate(
      (element) => document.activeElement === element,
    );
    expect(metadata.citation_current).toBe(true);
    if (goal === 'investigate_incident') {
      await panel
        .getByRole('button', { name: label('assistant.report.open'), exact: true })
        .click();
      try {
        await page.waitForFunction(
          () => document.querySelectorAll('.assistant__report tbody tr').length > 0,
          undefined,
          { timeout: 15_000 },
        );
      } catch {
        throw new Error('Real typed incident report did not settle');
      }
      const report = panel.locator('.assistant__report');
      metadata.report_rows = await report.locator('tbody tr').count();
      expect(metadata.report_rows >= 1 && metadata.report_rows <= 64).toBe(true);
      const firstRow = report.locator('tbody tr').first();
      const firstKind = await firstRow.getAttribute('data-kind');
      const focusTarget =
        firstKind === 'alert' || firstKind === 'workload'
          ? await firstRow.getAttribute('id')
          : 'resource-detail-name';
      expect(focusTarget).not.toBeNull();
      await report.locator('tbody button').first().click();
      try {
        await page.waitForFunction(
          (expected) => document.activeElement?.id === expected,
          focusTarget,
          { timeout: 5000 },
        );
      } catch {
        throw new Error('Real report citation authority focus did not settle');
      }
      metadata.report_current = true;
      await report
        .getByRole('button', { name: label('assistant.report.close'), exact: true })
        .click();
      expect(await report.locator('table').count()).toBe(0);
      metadata.report_cleared = true;
    }
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
    const historyRead = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/api/v1/assistant/history',
    );
    await page.goto('/assistant/history');
    const historyResponse = await historyRead;
    expect(historyResponse.status()).toBe(200);
    expect(historyResponse.headers()['cache-control']).toBe('no-store');
    await expect(page.locator('.assistant-history-row').first()).toBeVisible();
    const historyCount = await page.locator('.assistant-history-row').count();
    expect(historyCount >= 1 && historyCount <= 10).toBe(true);
    await expect(
      page.getByRole('heading', { name: label('assistant.history.title'), exact: true }),
    ).toBeVisible();
    await expect(page.getByText(question, { exact: true })).toHaveCount(0);
    expect(await page.locator('.assistant__text').count()).toBe(0);
    const selection = page.locator('.assistant-history-row input[type="checkbox"]').first();
    const selectedLabel = await selection.getAttribute('aria-label');
    if (selectedLabel === null) throw new Error('Private metadata selection unavailable');
    const selectedID = selectedLabel.slice(-67);
    expect(/^ar_[0-9a-f]{64}$/u.test(selectedID)).toBe(true);
    await selection.check();
    const pendingExport = page.waitForEvent('download');
    await page
      .getByRole('button', { name: label('assistant.history.export.action'), exact: true })
      .click();
    const exported = await pendingExport;
    expect(exported.suggestedFilename()).toBe('stackplore-run-history.json');
    const exportedStream = await exported.createReadStream();
    const exportedChunks: Buffer[] = [];
    let exportedBytes = 0;
    for await (const chunk of exportedStream) {
      const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk as Uint8Array);
      exportedBytes += bytes.length;
      if (exportedBytes > 32 * 1024) throw new Error('Private metadata export exceeded its bound');
      exportedChunks.push(bytes);
    }
    const exportText = Buffer.concat(exportedChunks).toString('utf8');
    const exportData = JSON.parse(exportText) as { schema_version?: unknown; entries?: unknown };
    expect(exportData.schema_version).toBe(1);
    if (!Array.isArray(exportData.entries) || exportData.entries.length !== 1)
      throw new Error('Private metadata selection changed');
    const exportEntry = exportData.entries[0] as Record<string, unknown>;
    expect(exportEntry.id).toBe(selectedID);
    expect(Object.keys(exportEntry).sort()).toEqual(
      [
        'id',
        'source_id',
        'resource_id',
        'goal',
        'state',
        'created_at',
        'updated_at',
        'range',
        'tool_calls',
        'inference_calls',
        'usage_uncertain',
      ].sort(),
    );
    expect(exportText.includes(question)).toBe(false);
    expect(admissions).toBe(1);
    await page.goto(resourcePage);
  } finally {
    page.off('request', observe);
    page.off('response', observeReply);

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
      resolve(`../../.tmp/hosted-live-${locale}-${role}-${goal}.json`),
      `${JSON.stringify(metadata)}\n`,
      { mode: 0o600 },
    );
    if (original !== null) await page.setViewportSize(original);
  }
}
