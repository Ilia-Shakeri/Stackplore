import type { ApiClient } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';
import { createHealthHistoryApi } from './health-history-api';
const resource = `r1_${'A'.repeat(43)}`;
const metadata = {
  availability: 'available',
  completeness: 'complete',
  observed_at: '2026-10-17T08:00:00.000300Z',
};
const sample = (id: string, time: string) => ({
  ...metadata,
  id,
  resource_id: resource,
  state: 'unknown',
  observed_at: time,
  synthetic: true,
});
describe('retained health transport', () => {
  it('preserves microsecond order and binds the exact resource query', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue({
      status: 200,
      requestId: 'request_health',
      data: {
        ...metadata,
        items: [
          sample('health_z', '2026-10-17T08:00:00.000200Z'),
          sample('health_a', '2026-10-17T08:00:00.000100Z'),
        ],
      },
    });
    const api = createHealthHistoryApi({ request: request as ApiClient['request'] });
    const result = await api.listHealthHistory({
      resourceId: resource,
      sourceId: 'source_demo',
      minutes: 60,
    });
    expect(result.items).toHaveLength(2);
    expect(request.mock.calls[0]?.[0]).toMatchObject({
      path: '/health/history',
      query: { resource_id: resource, source_id: 'source_demo', minutes: 60, limit: 200 },
    });
  });
  it.each(['cross-resource', 'duplicate', 'order', 'denied-data', 'partial-known'])(
    'rejects %s responses',
    async (kind) => {
      const first = sample('health_a', '2026-10-17T08:00:00.000200Z');
      const second = sample('health_b', '2026-10-17T08:00:00.000100Z');
      const body = { ...metadata, items: [first, second] };
      if (kind === 'cross-resource') first.resource_id = `r1_${'B'.repeat(43)}`;
      if (kind === 'duplicate') body.items[1] = first;
      if (kind === 'order') body.items.reverse();
      if (kind === 'denied-data') {
        body.availability = 'permission_denied';
        Object.assign(body, { reason_code: 'source.denied' });
      }
      if (kind === 'partial-known') {
        first.completeness = 'partial';
        first.state = 'healthy';
        Object.assign(first, { reason_code: 'source.partial' });
      }
      const request = vi
        .fn<ApiClient['request']>()
        .mockResolvedValue({ status: 200, requestId: 'request_health', data: body });
      await expect(
        createHealthHistoryApi({ request: request as ApiClient['request'] }).listHealthHistory({
          resourceId: resource,
        }),
      ).rejects.toThrow();
    },
  );
});
