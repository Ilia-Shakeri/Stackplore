import type { ApiClient } from '@stackplore/api-client';
import { expect, it, vi } from 'vitest';
import { createProductApi } from './product-api';

it('binds event filters to server history and rejects invalid text before transport', async () => {
  const request = vi.fn<ApiClient['request']>().mockResolvedValue({
    status: 200,
    requestId: 'request_events',
    data: {
      availability: 'available',
      completeness: 'complete',
      observed_at: '2026-08-16T08:00:00Z',
      items: [],
      page: { limit: 100, has_more: false },
    },
  });
  const api = createProductApi({ request: request as ApiClient['request'] });
  await api.listEvents({
    environmentId: 'env_shop',
    severity: 'error',
    search: 'pool',
    cursor: 'next_page',
  });
  expect(request).toHaveBeenCalledWith(
    expect.objectContaining({
      path: '/events',
      query: {
        limit: 100,
        cursor: 'next_page',
        environment_id: 'env_shop',
        severity: 'error',
        q: 'pool',
      },
    }),
  );
  request.mockClear();
  await expect(api.listEvents({ search: 'x'.repeat(201) })).rejects.toThrow();
  await expect(api.listEvents({ environmentId: '../private' })).rejects.toThrow();
  expect(request).not.toHaveBeenCalled();
});
