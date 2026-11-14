import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, type Page, type Request } from '@playwright/test';

const paths = ['/api/v1/resources', '/api/v1/topology', '/api/v1/metrics/current', '/api/v1/events'];
const pause = (ms: number) => new Promise<void>((done) => setTimeout(done, ms));

export function assertHostedWorkload() {
  if (process.env.GITHUB_ACTIONS !== 'true' || process.env.RUNNER_ENVIRONMENT !== 'github-hosted' || process.env.GITHUB_WORKFLOW !== 'stage9-workload' || process.env.SP_STAGE9_WORKLOAD !== 'true')
    throw new Error('Hosted workload runner required');
}

// Use the actual UI's source-scoped queries, without writing them into receipts.
export async function prepareWorkload(page: Page): Promise<Map<string, string>> {
  assertHostedWorkload();
  const resourcePage = page.url();
  const queries = new Map<string, string>();
  const observe = (request: Request) => {
    const url = new URL(request.url());
    if (request.method() === 'GET' && paths.includes(url.pathname))
      queries.set(url.pathname, `${url.pathname}${url.search}`);
  };
  page.on('request', observe);
  try {
    for (const route of ['/overview', '/resources', '/topology', '/events']) {
      await page.goto(route);
      await expect(page.locator('main h1')).toBeVisible();
      const path = route === '/overview' ? '/api/v1/metrics/current' : `/api/v1${route}`;
      await expect.poll(() => queries.has(path)).toBe(true);
      await pause(2500);
    }
  } finally {
    page.off('request', observe);
    await page.goto(resourcePage);
    await expect(page.locator('#resource-detail-name')).toBeVisible();
    await pause(2500);
  }
  return queries;
}

// Preserve the release workload: ten rounds, eight concurrent reads, 600 ms spacing.
export async function measureReads(page: Page, queries: Map<string, string>) {
  assertHostedWorkload();
  const durations: number[] = [];
  const started = Date.now();
  for (let round = 0; round < 10; round++) {
    const roundStarted = Date.now();
    await Promise.all(Array.from({ length: 8 }, async (_, index) => {
      const path = paths[index % paths.length];
      const query = path === undefined ? undefined : queries.get(path);
      if (query === undefined) throw new Error('Hosted workload query missing');
      const before = Date.now();
      const response = await page.request.get(query, { timeout: 10_000 });
      if (response.status() !== 200) throw new Error('Hosted workload read failed');
      const body = await response.json() as { availability?: unknown; completeness?: unknown; items?: unknown; page?: { has_more?: unknown } };
      if (!['available', 'stale'].includes(String(body.availability)) || !['complete', 'partial'].includes(String(body.completeness)) || !Array.isArray(body.items) || body.items.length > 200 || (path !== '/api/v1/metrics/current' && typeof body.page?.has_more !== 'boolean'))
        throw new Error('Hosted workload envelope invalid');
      durations.push(Date.now() - before);
    }));
    await pause(Math.max(0, 600 - (Date.now() - roundStarted)));
  }
  const sorted = [...durations].sort((a, b) => a - b);
  return { started_epoch_ms: started, finished_epoch_ms: Date.now(), requests: durations.length, parallel: 8, rounds: 10, spacing_ms: 600, latency_p50_ms: sorted[39] ?? 0, latency_p95_ms: sorted[75] ?? 0 };
}

export function workloadReceipt(name: 'load' | 'soak', value: object) {
  assertHostedWorkload();
  writeFileSync(resolve(`../../.tmp/hosted-${name}.json`), `${JSON.stringify({ source_revision: process.env.GITHUB_SHA, ...value })}\n`, { mode: 0o600 });
}

export async function realApplicationSoak(page: Page, label: (key: string) => string) {
  assertHostedWorkload();
  const queries = await prepareWorkload(page);
  const resourcePage = page.url();
  const started = Date.now();
  const report = { status: 'failed', scope: 'Real application, PostgreSQL and pinned processor alive; read workload and UI clear/reopen cycles, not process restart or active generation soak', duration_ms: 0, expected_cycles: 100, completed_cycles: 0, successful_reads: 0, health_checks: 0, cleared_views: 0, max_read_p95_ms: 0, samples: [] as { cycle: number; elapsed_ms: number; read_p95_ms: number }[] };
  try {
    for (let cycle = 0; cycle < 100; cycle++) {
      await page.getByRole('button', { name: label('assistant.open'), exact: true }).click();
      const panel = page.locator('.assistant');
      await expect(panel.getByRole('textbox', { name: label('assistant.question'), exact: true })).toHaveValue('');
      if (await panel.locator('.assistant__text').count() !== 0) throw new Error('Soak retained private reply');
      await panel.getByRole('button', { name: label('assistant.close'), exact: true }).click();
      await expect(panel.getByRole('textbox')).toHaveCount(0);
      await expect(panel.locator('.assistant__text')).toHaveCount(0);
      report.cleared_views++;
      await page.goto('about:blank');
      await pause(2500);
      const batch = await measureReads(page, queries);
      report.successful_reads += batch.requests;
      report.max_read_p95_ms = Math.max(report.max_read_p95_ms, batch.latency_p95_ms);
      for (const target of ['/readyz', 'http://127.0.0.1:58102/health']) {
        const response = await page.request.get(target, { timeout: 5000 });
        if (response.status() !== 200) throw new Error('Soak owned service health failed');
        report.health_checks++;
      }
      await page.goto(resourcePage);
      await expect(page.locator('#resource-detail-name')).toBeVisible();
      report.completed_cycles++;
      report.samples.push({ cycle: cycle + 1, elapsed_ms: Date.now() - started, read_p95_ms: batch.latency_p95_ms });
      workloadReceipt('soak', { ...report, duration_ms: Date.now() - started });
      await pause(Math.max(0, started + (cycle + 1) * 36_000 - Date.now()));
    }
    if (Date.now() - started < 3_600_000) throw new Error('Soak duration incomplete');
    report.status = 'passed';
  } finally {
    report.duration_ms = Date.now() - started;
    workloadReceipt('soak', report);
  }
}
