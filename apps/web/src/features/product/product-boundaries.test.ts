import { createApiClient, type ProductEventPage } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import { createBrowserApiClient } from '../../app/api-client';
import { createProductApi, type SessionActionRunner } from './product-api';

const observedAt = '2026-08-16T08:00:00Z';
const requestId = 'req_product_boundaries';
const resourceId = `r1_${'A'.repeat(43)}`;
const headers = { 'Content-Type': 'application/json', 'X-Request-ID': requestId };
const eventPage = (summary: string, count = 1): ProductEventPage => ({
  availability: 'available',
  completeness: 'complete',
  observed_at: observedAt,
  items: Array.from({ length: count }, (_, index) => ({
    id: `event_safe_${String(index)}`,
    kind: 'demo.transition',
    severity: 'info',
    reason_code: 'demo.transition',
    summary,
    occurred_at: observedAt,
    observed_at: observedAt,
    synthetic: true,
  })),
  page: { limit: 100, has_more: false },
});
const runner: SessionActionRunner = {
  runRecentAction: async (_purpose, _password, action) =>
    action({ kind: 'session', csrfToken: 'A'.repeat(43) }),
  runSessionAction: async (action) => action({ kind: 'session', csrfToken: 'A'.repeat(43) }),
};
const fixtureClient = (value: unknown) => {
  const fetch = vi
    .fn<typeof globalThis.fetch>()
    .mockImplementation(() => Promise.resolve(new Response(JSON.stringify(value), { headers })));
  return {
    fetch,
    api: createProductApi(
      createBrowserApiClient({ fetch, requestIdFactory: () => requestId }),
      runner,
    ),
  };
};

describe('production product transport boundaries', () => {
  it.each(['generic_webhook', 'matrix'] as const)(
    'sends the %s test through the real client',
    async (adapter) => {
      const queued = {
        notification_id: 'notification_safe',
        destination_reference_id: 'destination_safe',
        state: 'queued',
      };
      const fixture = fixtureClient(queued);
      await expect(fixture.api.testNotification('', adapter)).resolves.toEqual(queued);
      expect(fixture.fetch).toHaveBeenCalledOnce();
      expect(fixture.fetch.mock.calls[0]?.[0]).toBe(
        `/api/v1/settings/notifications/test${adapter === 'matrix' ? '?adapter=matrix' : ''}`,
      );
    },
  );

  it.each([4097, 8192])(
    'accepts an event summary with %s supplementary code points',
    async (length) => {
      const summary = '🚀'.repeat(length);
      await expect(fixtureClient(eventPage(summary)).api.listEvents()).resolves.toMatchObject({
        items: [{ summary }],
      });
    },
  );

  it('keeps the 8192-code-point summary maximum closed', async () => {
    await expect(fixtureClient(eventPage('🚀'.repeat(8193))).api.listEvents()).rejects.toThrow(
      'Event response is invalid.',
    );
  });

  it('accepts 200 search code points and rejects 201 before transport', async () => {
    const fixture = fixtureClient(eventPage('safe'));
    await expect(fixture.api.listEvents({ search: '🚀'.repeat(200) })).resolves.toMatchObject({
      items: [{ summary: 'safe' }],
    });
    const url = fixture.fetch.mock.calls[0]?.[0];
    if (typeof url !== 'string') throw new Error('The request URL is missing.');
    expect(new URL(url, 'http://localhost').searchParams.get('q')).toBe('🚀'.repeat(200));
    await expect(fixture.api.listEvents({ search: '🚀'.repeat(201) })).rejects.toThrow(
      'Event search is invalid.',
    );
    expect(fixture.fetch).toHaveBeenCalledOnce();
  });

  it('accepts 256-code-point display names and scopes but rejects 257', async () => {
    const resource = {
      id: resourceId,
      source_id: 'source_demo',
      provider: 'demo',
      environment_id: 'env_demo',
      kind: 'demo.gateway',
      scope_kind: 'project',
      scope_name: '🚀'.repeat(256),
      name: '🚀'.repeat(256),
      synthetic: true,
      observed_at: observedAt,
    };
    const page = {
      availability: 'available',
      completeness: 'complete',
      observed_at: observedAt,
      items: [resource],
      page: { limit: 100, has_more: false },
    };
    await expect(
      fixtureClient(page).api.listResources({ scopeName: resource.scope_name }),
    ).resolves.toMatchObject({ items: [resource] });
    await expect(
      fixtureClient({
        ...page,
        items: [{ ...resource, name: '🚀'.repeat(257) }],
      }).api.listResources(),
    ).rejects.toThrow('Resource response is invalid.');
  });

  it.each([true, false])(
    'reads a legal multilingual page above one MiB (length header %s)',
    async (withLength) => {
      const page = eventPage('ن'.repeat(8192), 100);
      const text = JSON.stringify(page);
      const size = new TextEncoder().encode(text).byteLength;
      expect(size).toBeGreaterThan(1024 * 1024);
      const fetch = vi.fn<typeof globalThis.fetch>().mockImplementation(() =>
        Promise.resolve(
          new Response(text, {
            headers: { ...headers, ...(withLength ? { 'Content-Length': String(size) } : {}) },
          }),
        ),
      );
      await expect(
        createProductApi(
          createBrowserApiClient({ fetch, requestIdFactory: () => requestId }),
        ).listEvents(),
      ).resolves.toEqual(page);
      const generic = createApiClient({ fetch, requestIdFactory: () => requestId });
      await expect(generic.request({ path: '/events' })).rejects.toMatchObject({
        code: 'response_too_large',
      });
    },
  );

  it.each([true, false])(
    'cancels production JSON above 16 MiB (length header %s)',
    async (withLength) => {
      const cancel = vi.fn();
      const body = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(new Uint8Array(16 * 1024 * 1024));
          controller.enqueue(new Uint8Array(1));
        },
        cancel,
      });
      const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue(
        new Response(body, {
          headers: {
            ...headers,
            ...(withLength ? { 'Content-Length': String(16 * 1024 * 1024 + 1) } : {}),
          },
        }),
      );
      await expect(
        createBrowserApiClient({ fetch, requestIdFactory: () => requestId }).request({
          path: '/events',
        }),
      ).rejects.toMatchObject({ code: 'response_too_large' });
      expect(cancel).toHaveBeenCalledOnce();
    },
  );

  it('reads a legal log page above one MiB through the production client', async () => {
    const page = {
      ...eventPage(''),
      items: Array.from({ length: 100 }, (_, index) => ({
        id: `log_safe_${String(index)}`,
        resource_id: resourceId,
        stream: 'stdout',
        sequence: index + 1,
        severity: 'info',
        timestamp: observedAt,
        message: 'x'.repeat(16_384),
        redacted: false,
        truncated: false,
        synthetic: true,
      })),
    };
    expect(new TextEncoder().encode(JSON.stringify(page)).byteLength).toBeGreaterThan(1024 * 1024);
    await expect(fixtureClient(page).api.listLogs()).resolves.toEqual(page);
  });

  it('retains the one-MiB download cap with the larger production JSON budget', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue(
      new Response('safe', {
        headers: {
          ...headers,
          'Content-Type': 'text/plain',
          'Content-Length': String(1024 * 1024 + 1),
        },
      }),
    );
    await expect(
      createProductApi(
        createBrowserApiClient({ fetch, requestIdFactory: () => requestId }),
      ).downloadLogs(),
    ).rejects.toMatchObject({ code: 'response_too_large' });
  });
});
