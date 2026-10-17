import type { ApiClient } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import { createProductApi, type SessionActionRunner } from './product-api';
import type { SessionProtection } from '../auth/auth-api';

const resourceId = `r1_${'A'.repeat(43)}`;
const observedAt = '2026-08-16T08:00:00Z';
const requestId = 'req_product_01';

const response = (data: unknown) => ({ data, requestId, status: 200 });
const logDownload = (footer = 'complete') =>
  `# Stackplore bounded log download v1\n${observedAt} info ${resourceId} safe redacted text\n# stackplore-export: ${footer}\n`;
const observation = () => ({
  availability: 'available',
  completeness: 'complete',
  observed_at: observedAt,
});

const resource = () => ({
  environment_id: 'env_demo_shop',
  health: 'degraded',
  id: resourceId,
  kind: 'demo.gateway',
  name: 'Gateway',
  observed_at: observedAt,
  provider: 'demo',
  scope_kind: 'project',
  scope_name: 'demo-shop',
  source_id: 'source_demo',
  synthetic: true,
});

const source = () => ({
  availability: 'available',
  completeness: 'complete',
  health: { degraded: 1, healthy: 4, unhealthy: 1, unknown: 0 },
  id: 'source_demo',
  name: 'Demo shop',
  observed_at: observedAt,
  provider: 'demo',
  resource_count: 6,
  synthetic: true,
});

const alertRule = () => ({
  created_at: observedAt,
  display_name: 'Provider lost',
  enabled: true,
  evaluation_interval_seconds: 30,
  id: 'rule_provider_lost',
  kind: 'provider_connection_lost',
  pending_for_seconds: 60,
  severity: 'critical',
  source_id: 'source_demo',
  updated_at: observedAt,
  version: 1,
});

describe('product API adapter', () => {
  it('downloads a fixed filtered selection as bounded text without pagination parameters', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response(logDownload('limited')));
    const api = createProductApi({ request: request as ApiClient['request'] });
    const controller = new AbortController();
    await expect(
      api.downloadLogs({
        sourceId: 'source_kube',
        resourceId,
        from: '2026-08-16T07:00:00Z',
        to: observedAt,
        container: 'api-v2',
        previous: true,
        search: 'worker',
        severity: 'warning',
        signal: controller.signal,
      }),
    ).resolves.toEqual({ text: logDownload('limited'), limited: true });
    expect(request).toHaveBeenCalledWith({
      path: '/logs/download',
      responseType: 'text',
      signal: controller.signal,
      query: {
        source_id: 'source_kube',
        resource_id: resourceId,
        from: '2026-08-16T07:00:00Z',
        to: observedAt,
        container: 'api-v2',
        previous: true,
        q: 'worker',
        severity: 'warning',
      },
    });
  });

  it.each([
    logDownload('incomplete'),
    logDownload().replace('# stackplore-export: complete\n', ''),
    logDownload().replace(`${observedAt} info`, 'forged info'),
    `# Stackplore bounded log download v1\n${`${observedAt} info ${resourceId} safe\n`.repeat(1001)}# stackplore-export: complete\n`,
  ])('rejects an incomplete or malformed download without a usable file', async (text) => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(text));
    await expect(
      createProductApi({ request: request as ApiClient['request'] }).downloadLogs(),
    ).rejects.toThrow('Log download is incomplete or invalid.');
  });

  it('rejects a response for another source or resource', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(
        response({ ...observation(), items: [resource()], page: { has_more: false, limit: 50 } }),
      )
      .mockResolvedValueOnce(response(resource()));
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.listResources({ sourceId: 'source_other' })).rejects.toThrow(
      'Resource source is invalid.',
    );
    await expect(api.getResource(`r1_${'C'.repeat(43)}`)).rejects.toThrow(
      'Resource response is invalid.',
    );
  });

  it('accepts bounded topology provenance and rejects absent or corrupted evidence', async () => {
    const evidence = {
      id: 'evidence_demo',
      provider: 'demo',
      source_id: 'source_demo',
      provenance: 'declared_dependency',
      certainty: 'explicit',
      confidence: 1,
      observed_at: observedAt,
      synthetic: true,
    };
    const edge = {
      id: `rel1_${'A'.repeat(43)}`,
      from: resourceId,
      to: `r1_${'C'.repeat(43)}`,
      kind: 'depends_on',
      direction: 'source_to_target',
      observed_at: observedAt,
      synthetic: true,
      confidence: 1,
      evidence: [evidence],
    };
    const payload = (evidenceItems: unknown) => ({
      ...observation(),
      items: [{ ...edge, evidence: evidenceItems }],
      page: { has_more: false, limit: 100 },
    });
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(payload([evidence])));
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.listTopology()).resolves.toMatchObject({
      items: [{ evidence: [{ provenance: 'declared_dependency' }] }],
    });
    for (const evidenceItems of [
      [],
      [evidence, evidence],
      [{ ...evidence, confidence: 2 }],
      [{ ...evidence, provenance: 'guess' }],
      [{ ...evidence, hidden: true }],
      Array.from({ length: 33 }, (_, index) => ({ ...evidence, id: `evidence_${String(index)}` })),
    ]) {
      request.mockResolvedValueOnce(response(payload(evidenceItems)));
      await expect(api.listTopology()).rejects.toThrow('Topology response is invalid.');
    }
  });

  it('owns every inventory filter and continuation value', async () => {
    const controller = new AbortController();
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(
        response({ ...observation(), items: [], page: { has_more: false, limit: 25 } }),
      );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await api.listResources({
      availability: 'stale',
      cursor: 'next_page',
      direction: 'desc',
      environmentId: 'env_shop',
      health: 'degraded',
      kind: 'docker.container',
      limit: 25,
      provider: 'docker',
      scopeKind: 'project',
      scopeName: 'shop',
      search: 'worker',
      signal: controller.signal,
      sort: 'observed_at',
      sourceId: 'source_docker',
    });
    expect(request).toHaveBeenCalledWith({
      path: '/resources',
      signal: controller.signal,
      query: {
        availability: 'stale',
        cursor: 'next_page',
        direction: 'desc',
        environment_id: 'env_shop',
        health: 'degraded',
        kind: 'docker.container',
        limit: 25,
        provider: 'docker',
        scope_kind: 'project',
        scope_name: 'shop',
        search: 'worker',
        sort: 'observed_at',
        source_id: 'source_docker',
      },
    });
  });

  it('keeps event and log cursor, resource, range and container values together', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(
        response({ ...observation(), items: [], page: { has_more: false, limit: 25 } }),
      );
    const api = createProductApi({ request: request as ApiClient['request'] });
    const selection = {
      cursor: 'next_page',
      from: '2026-08-16T07:00:00Z',
      limit: 25,
      resourceId,
      sourceId: 'source_kube',
      to: observedAt,
    };
    await api.listEvents(selection);
    await api.listLogs({ ...selection, container: 'api-v2', previous: true });
    expect(request.mock.calls.map(([input]) => input.query)).toEqual([
      {
        cursor: 'next_page',
        from: selection.from,
        limit: 25,
        resource_id: resourceId,
        source_id: 'source_kube',
        to: observedAt,
      },
      {
        container: 'api-v2',
        cursor: 'next_page',
        from: selection.from,
        limit: 25,
        previous: true,
        resource_id: resourceId,
        source_id: 'source_kube',
        to: observedAt,
      },
    ]);
  });

  it('uses an explicit history range without an implicit minutes filter', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response({ ...observation(), items: [] }));
    const api = createProductApi({ request: request as ApiClient['request'] });
    await api.listMetricHistory({
      from: '2026-08-16T07:00:00Z',
      to: observedAt,
      resourceId,
      limit: 40,
    });
    expect(request).toHaveBeenCalledWith({
      path: '/metrics/history',
      query: {
        from: '2026-08-16T07:00:00Z',
        to: observedAt,
        resource_id: resourceId,
        limit: 40,
      },
    });
    await expect(
      api.listMetricHistory({ minutes: 60, from: '2026-08-16T07:00:00Z', to: observedAt }),
    ).rejects.toThrow('Metric history range is invalid.');
    expect(request).toHaveBeenCalledTimes(1);
  });

  it.each([
    { from: observedAt },
    { from: '2026-02-30T00:00:00Z', to: observedAt },
    { from: observedAt, to: observedAt },
    { from: '2026-08-16T08:00:00.0000001Z', to: '2026-08-16T08:00:01Z' },
    { from: '2026-08-01T08:00:00Z', to: observedAt },
    { from: '2026-08-09T08:00:00Z', to: '2026-08-16T08:00:00.000001Z' },
  ])('rejects an invalid or oversized log range before reading %j', async (range) => {
    const request = vi.fn<ApiClient['request']>();
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.listLogs(range)).rejects.toThrow('Time range is invalid.');
    expect(request).not.toHaveBeenCalled();
  });

  it('retains sub-millisecond ranges and rejects bad page and container selections', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(
        response({ ...observation(), items: [], page: { has_more: false, limit: 100 } }),
      );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(
      api.listLogs({ from: '2026-08-16T08:00:00Z', to: '2026-08-16T08:00:00.000001Z' }),
    ).resolves.toMatchObject({ items: [] });
    for (const options of [
      { limit: 201 },
      { limit: 0 },
      { cursor: 'bad cursor' },
      { resourceId: 'bad' },
      { previous: true },
      { container: 'api' },
      { resourceId, container: '../api' },
    ]) {
      await expect(api.listLogs(options)).rejects.toThrow();
    }
    expect(request).toHaveBeenCalledTimes(1);
  });

  it.each(['unsupported', 'unavailable', 'permission_denied'])(
    'rejects resource data under a forbidden %s status',
    async (availability) => {
      const request = vi.fn<ApiClient['request']>().mockResolvedValue(
        response({
          ...observation(),
          availability,
          reason_code: 'provider.read.failed',
          items: [resource()],
          page: { has_more: false, limit: 50 },
        }),
      );
      const api = createProductApi({ request: request as ApiClient['request'] });
      await expect(api.listResources()).rejects.toThrow('Resource page is invalid.');
    },
  );

  it('keeps stale partial evidence, rejects unknown fields and invalid calendar timestamps', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        ...observation(),
        availability: 'stale',
        completeness: 'partial',
        reason_code: 'provider.capture.failed',
        items: [resource()],
        page: { has_more: false, limit: 50 },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.listResources()).resolves.toMatchObject({
      availability: 'stale',
      completeness: 'partial',
      items: [{ id: resourceId }],
    });
    request.mockResolvedValueOnce(response({ ...resource(), observed_at: '2026-02-30T08:00:00Z' }));
    await expect(api.getResource(resourceId)).rejects.toThrow('Resource response is invalid.');
    request.mockResolvedValueOnce(response({ ...resource(), secret: 'hidden' }));
    await expect(api.getResource(resourceId)).rejects.toThrow('Resource response is invalid.');
  });

  it('manages alert rules with session proof and strong versions', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(response(alertRule()))
      .mockResolvedValueOnce(response({ ...alertRule(), enabled: false, version: 2 }))
      .mockResolvedValueOnce(response(undefined));
    const runner: SessionActionRunner = {
      runSessionAction: async (action) =>
        await action({ csrfToken: 'A'.repeat(43), kind: 'session' }),
    };
    const api = createProductApi({ request: request as ApiClient['request'] }, runner);
    const input = {
      display_name: 'Provider lost',
      enabled: true,
      evaluation_interval_seconds: 30,
      kind: 'provider_connection_lost' as const,
      pending_for_seconds: 60,
      severity: 'critical' as const,
      source_id: 'source_demo',
    };

    await expect(api.createAlertRule(input)).resolves.toMatchObject({ version: 1 });
    await expect(
      api.replaceAlertRule('rule_provider_lost', 1, { ...input, enabled: false }),
    ).resolves.toMatchObject({ version: 2 });
    await expect(api.deleteAlertRule('rule_provider_lost', 2)).resolves.toBeUndefined();
    expect(request).toHaveBeenNthCalledWith(2, {
      body: { ...input, enabled: false },
      headers: { 'If-Match': '"1"' },
      method: 'PUT',
      path: '/alert-rules/rule_provider_lost',
      protection: { csrfToken: 'A'.repeat(43), kind: 'session' },
    });
    expect(request).toHaveBeenNthCalledWith(3, {
      headers: { 'If-Match': '"2"' },
      method: 'DELETE',
      path: '/alert-rules/rule_provider_lost',
      protection: { csrfToken: 'A'.repeat(43), kind: 'session' },
    });
  });

  it('lists only strict bounded alert-rule records', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response({ items: [alertRule()], page: { has_more: false, limit: 100 } }));
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.listAlertRules()).resolves.toMatchObject({
      items: [{ id: 'rule_provider_lost' }],
    });
    expect(request).toHaveBeenCalledWith({ path: '/alert-rules', query: { limit: 100 } });
  });

  it('acknowledges through current session protection', async () => {
    const alert = {
      acknowledged_at: observedAt,
      id: 'alert_one',
      rule_id: 'rule_one',
      severity: 'warning',
      started_at: observedAt,
      state: 'acknowledged',
      summary_code: 'alert.resource_unhealthy.matched',
      synthetic: false,
      updated_at: observedAt,
    };
    const request = vi.fn().mockResolvedValue(response(alert));
    const protections: SessionProtection[] = [];
    const runner: SessionActionRunner = {
      async runSessionAction<Result>(
        action: (protection: SessionProtection) => Promise<Result>,
      ): Promise<Result> {
        const protection: SessionProtection = {
          csrfToken: 'A'.repeat(43),
          kind: 'session',
        };
        protections.push(protection);
        return await action(protection);
      },
    };
    const api = createProductApi({ request: request as ApiClient['request'] }, runner);

    await expect(api.acknowledgeAlert(alert.id)).resolves.toMatchObject({
      state: 'acknowledged',
    });
    expect(protections).toHaveLength(1);
    expect(request).toHaveBeenCalledWith({
      method: 'POST',
      path: '/alerts/alert_one/acknowledge',
      protection: { csrfToken: 'A'.repeat(43), kind: 'session' },
    });
  });

  it('parses a linked notification page without adding source selection', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        items: [
          {
            alert_id: 'alert_demo_checkout',
            created_at: observedAt,
            id: 'notification_demo_checkout',
            kind: 'notification.alert_firing',
            resource_id: resourceId,
            severity: 'critical',
            source_id: 'source_demo',
            summary_code: 'demo.alert.checkout_unhealthy',
            synthetic: true,
          },
        ],
        page: { has_more: false, limit: 50 },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.listNotifications()).resolves.toMatchObject({
      items: [{ id: 'notification_demo_checkout' }],
    });
    expect(request).toHaveBeenCalledWith({ path: '/notifications', query: { limit: 50 } });
  });

  it('parses the exact ordered readiness report', async () => {
    const report = {
      checks: [
        { name: 'configuration', state: 'ready' },
        { name: 'database', state: 'not_ready' },
        { name: 'service', state: 'ready' },
      ],
      state: 'not_ready',
    };
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(report));
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.getSystemReadiness()).resolves.toEqual(report);
    expect(request).toHaveBeenCalledWith({ path: '/system/readiness' });
  });

  it('accepts only secret-free notification settings', async () => {
    const status = {
      adapter: 'generic_webhook',
      matrix: { adapter: 'matrix', enabled: false },
      configuration_source: 'operator_environment',
      delivery_semantics: 'at_least_once',
      destination_reference_id: 'webhook_primary',
      enabled: true,
    };
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(status));
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.getNotificationSettings()).resolves.toEqual(status);
    expect(request).toHaveBeenCalledWith({ path: '/settings/notifications' });

    request.mockResolvedValueOnce(response({ ...status, url: 'https://example.test/hook' }));
    await expect(api.getNotificationSettings()).rejects.toThrow(
      'Notification settings response is invalid.',
    );
    for (const matrix of [
      {
        adapter: 'matrix',
        enabled: true,
        destination_reference_id: 'matrix_primary',
        access_token: 'fixture-only',
      },
      {
        adapter: 'matrix',
        enabled: true,
        destination_reference_id: 'matrix_primary',
        url: 'https://example.test',
      },
      { adapter: 'matrix', enabled: true },
      { adapter: 'matrix', enabled: false, destination_reference_id: 'matrix_primary' },
    ]) {
      request.mockResolvedValueOnce(response({ ...status, matrix }));
      await expect(api.getNotificationSettings()).rejects.toThrow(
        'Notification settings response is invalid.',
      );
    }
  });

  it('queues a notification test with an exact one-use recent proof', async () => {
    const queued = {
      destination_reference_id: 'webhook_primary',
      notification_id: 'notification_test_01',
      state: 'queued' as const,
    };
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(queued));
    const calls: Array<[string, string]> = [];
    const runner: SessionActionRunner = {
      runRecentAction: async (purpose, password, action) => {
        calls.push([purpose, password]);
        return await action({
          csrfToken: 'A'.repeat(43),
          kind: 'session',
          recentAuthToken: 'B'.repeat(43),
        });
      },
      runSessionAction: async (action) =>
        await action({ csrfToken: 'A'.repeat(43), kind: 'session' }),
    };
    const api = createProductApi({ request: request as ApiClient['request'] }, runner);

    await expect(api.testNotification('admin-password')).resolves.toEqual(queued);
    expect(calls).toEqual([['notification.test', 'admin-password']]);
    expect(request).toHaveBeenCalledWith({
      method: 'POST',
      path: '/settings/notifications/test',
      protection: {
        csrfToken: 'A'.repeat(43),
        kind: 'session',
        recentAuthToken: 'B'.repeat(43),
      },
    });
    await expect(api.testNotification('admin-password', 'matrix')).resolves.toEqual(queued);
    expect(request).toHaveBeenLastCalledWith({
      method: 'POST',
      path: '/settings/notifications/test?adapter=matrix',
      protection: { csrfToken: 'A'.repeat(43), kind: 'session', recentAuthToken: 'B'.repeat(43) },
    });
  });

  it('parses a bounded resource page and owns search query values', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        ...observation(),
        items: [{ ...resource(), source_id: 'source_cluster' }],
        page: { has_more: false, limit: 50 },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(
      api.listResources({ search: 'gate', sourceId: 'source_cluster' }),
    ).resolves.toMatchObject({
      items: [{ id: resourceId, name: 'Gateway' }],
    });
    expect(request).toHaveBeenCalledWith({
      path: '/resources',
      query: { limit: 50, search: 'gate', source_id: 'source_cluster' },
    });
  });

  it('loads exact resource and health routes', async () => {
    const health = {
      observed_at: observedAt,
      reason_codes: ['demo.health.latency'],
      resource_id: resourceId,
      state: 'degraded',
      synthetic: true,
    };
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(response(resource()))
      .mockResolvedValueOnce(response(health));
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.getResource(resourceId)).resolves.toMatchObject({ id: resourceId });
    await expect(api.getResourceHealth(resourceId)).resolves.toEqual(health);
    expect(request.mock.calls.map(([input]) => input.path)).toEqual([
      `/resources/${resourceId}`,
      `/resources/${resourceId}/health`,
    ]);
  });

  it('parses a bounded environment page', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        ...observation(),
        items: [
          {
            availability: 'available',
            completeness: 'complete',
            id: 'env_demo_shop',
            name: 'Demo production',
            observed_at: observedAt,
            provider: 'demo',
            source_id: 'source_demo',
            synthetic: true,
          },
        ],
        page: { has_more: false, limit: 50 },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.listEnvironments()).resolves.toMatchObject({
      items: [{ id: 'env_demo_shop' }],
    });
    expect(request).toHaveBeenCalledWith({ path: '/environments', query: { limit: 50 } });
  });

  it('parses protected audit facts and owns the cursor', async () => {
    const cursor = 'next_audit_page';
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        items: [
          {
            action: 'auth.login',
            actor: { id: 'user_admin', kind: 'user' },
            id: 'audit_login_01',
            occurred_at: observedAt,
            outcome: 'success',
            request_id: 'request_login_01',
            summary_code: 'auth.login_succeeded',
            target: { id: 'user_admin', kind: 'auth.account' },
          },
        ],
        page: { has_more: false, limit: 50 },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.listAudit({ cursor })).resolves.toMatchObject({
      items: [{ action: 'auth.login', outcome: 'success' }],
    });
    expect(request).toHaveBeenCalledWith({
      path: '/audit',
      query: { cursor, limit: 50 },
    });
  });

  it('rejects drifted and unbounded payloads', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(response({ ...resource(), hidden: true }))
      .mockResolvedValueOnce(response({ ...source(), resource_count: -1 }));
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.getResource(resourceId)).rejects.toThrow('Resource response is invalid.');
    await expect(api.listSources()).rejects.toThrow('Source response is invalid.');
  });

  it.each([1025, 4096])('accepts bounded observation cursor length %s', async (length) => {
    const cursor = 'A'.repeat(length);
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        ...observation(),
        items: [resource()],
        page: { has_more: true, limit: 50, next_cursor: cursor },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });
    const page = await api.listResources();
    expect(page.page.has_more).toBe(true);
    if (!page.page.has_more) throw new Error('Expected another observation page.');
    expect(page.page.next_cursor).toBe(cursor);
  });

  it('rejects an observation cursor beyond the shared cap', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        ...observation(),
        items: [resource()],
        page: { has_more: true, limit: 50, next_cursor: 'A'.repeat(4097) },
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.listResources()).rejects.toThrow('Resource page is invalid.');
  });

  it('rejects malformed identifiers before transport', async () => {
    const request = vi.fn<ApiClient['request']>();
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.getResource('bad')).rejects.toThrow('Resource identifier is invalid.');
    expect(request).not.toHaveBeenCalled();
  });

  it('rejects malformed source scope before transport', async () => {
    const request = vi.fn<ApiClient['request']>();
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.listResources({ sourceId: 'bad source' })).rejects.toThrow(
      'Source identifier is invalid.',
    );
    expect(request).not.toHaveBeenCalled();
  });

  it('keeps each concurrent read bound to its own source', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(
        response({ ...observation(), items: [], page: { has_more: false, limit: 50 } }),
      );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await Promise.all([
      api.listResources({ sourceId: 'source_demo' }),
      api.listResources({ sourceId: 'source_docker' }),
      api.listResources(),
    ]);
    expect(request.mock.calls.map(([input]) => input.query)).toEqual([
      { limit: 50, source_id: 'source_demo' },
      { limit: 50, source_id: 'source_docker' },
      { limit: 50 },
    ]);
  });

  it.each([
    { name: 'unknown.metric' },
    { name: 'cpu.usage.cores', unit: 'bytes' },
    { value: Number.MAX_SAFE_INTEGER + 1 },
    { interval_seconds: 86_401 },
    { name: 'restart.count', unit: 'count', value: 1.5 },
    { sampled_at: '2026-08-16' },
  ])('rejects metric contract drift %j', async (override) => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        ...observation(),
        items: [
          {
            id: 'metric_cpu',
            resource_id: resourceId,
            name: 'cpu.usage.cores',
            unit: 'cores',
            value: 0.5,
            sampled_at: observedAt,
            interval_seconds: 30,
            synthetic: true,
            ...override,
          },
        ],
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.listMetrics()).rejects.toThrow('Metric response is invalid.');
  });

  it.each([
    Array.from({ length: 33 }, () => 'health.unhealthy'),
    ['no namespace'],
    [`health.${'a'.repeat(91)}`],
  ])('rejects health reason contract drift %j', async (reason_codes) => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        observed_at: observedAt,
        reason_codes,
        resource_id: resourceId,
        state: 'degraded',
        synthetic: true,
      }),
    );
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.getResourceHealth(resourceId)).rejects.toThrow(
      'Resource health response is invalid.',
    );
  });

  it('parses bounded topology, log, metric, event, and alert reads', async () => {
    const page = { has_more: false, limit: 100 };
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(
        response({
          ...observation(),
          items: [
            {
              confidence: 1,
              evidence: [
                {
                  id: 'evidence_demo',
                  provider: 'demo',
                  source_id: 'source_demo',
                  provenance: 'declared_dependency',
                  certainty: 'explicit',
                  confidence: 1,
                  observed_at: observedAt,
                  synthetic: true,
                },
              ],
              direction: 'source_to_target',
              from: resourceId,
              id: `rel1_${'B'.repeat(43)}`,
              kind: 'depends_on',
              observed_at: observedAt,
              synthetic: true,
              to: `r1_${'C'.repeat(43)}`,
            },
          ],
          page,
        }),
      )
      .mockResolvedValueOnce(
        response({
          ...observation(),
          items: [
            {
              id: 'log_demo_1',
              message: 'request done',
              redacted: false,
              resource_id: resourceId,
              sequence: 1,
              severity: 'info',
              stream: 'stdout',
              synthetic: true,
              timestamp: observedAt,
              truncated: false,
            },
          ],
          page,
        }),
      )
      .mockResolvedValueOnce(
        response({
          ...observation(),
          items: [
            {
              id: 'metric_demo_1_1',
              interval_seconds: 30,
              name: 'cpu.usage.cores',
              resource_id: resourceId,
              sampled_at: observedAt,
              synthetic: true,
              unit: 'cores',
              value: 0.25,
            },
          ],
          observed_at: observedAt,
        }),
      )
      .mockResolvedValueOnce(
        response({
          availability: 'available',
          completeness: 'partial',
          items: [
            {
              id: 'metric_history_01_01_01',
              interval_seconds: 300,
              name: 'cpu.usage.cores',
              resource_id: resourceId,
              sampled_at: observedAt,
              synthetic: true,
              unit: 'cores',
              value: 0.3,
            },
          ],
          observed_at: observedAt,
          reason_code: 'demo.metric.downsampled',
        }),
      )
      .mockResolvedValueOnce(
        response({
          ...observation(),
          items: [
            {
              id: 'event_demo_1',
              kind: 'demo.health.changed',
              observed_at: observedAt,
              occurred_at: observedAt,
              reason_code: 'demo.health.observed',
              resource_id: resourceId,
              severity: 'info',
              summary: 'ready',
              synthetic: true,
            },
          ],
          page,
        }),
      )
      .mockResolvedValueOnce(
        response({
          items: [
            {
              environment_id: 'env_demo_shop',
              id: 'alert_demo_checkout',
              resource_id: resourceId,
              rule_id: 'rule_demo_checkout_health',
              severity: 'critical',
              started_at: observedAt,
              state: 'firing',
              summary_code: 'demo.alert.checkout_unhealthy',
              synthetic: true,
              updated_at: observedAt,
            },
          ],
          page,
        }),
      );
    const api = createProductApi({ request: request as ApiClient['request'] });

    await expect(api.listTopology()).resolves.toMatchObject({ items: [{ kind: 'depends_on' }] });
    await expect(api.listLogs()).resolves.toMatchObject({ items: [{ message: 'request done' }] });
    await expect(api.listMetrics()).resolves.toMatchObject({ items: [{ value: 0.25 }] });
    await expect(api.listMetricHistory()).resolves.toMatchObject({
      completeness: 'partial',
      items: [{ value: 0.3 }],
    });
    await expect(api.listEvents()).resolves.toMatchObject({ items: [{ summary: 'ready' }] });
    await expect(api.listAlerts()).resolves.toMatchObject({ items: [{ state: 'firing' }] });
    expect(request).toHaveBeenLastCalledWith({ path: '/alerts', query: { limit: 100 } });
  });
});
