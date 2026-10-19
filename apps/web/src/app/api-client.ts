import { createApiClient, type ApiClientOptions } from '@stackplore/api-client';

export function createBrowserApiClient(
  options: Pick<ApiClientOptions, 'fetch' | 'requestIdFactory'> = {},
) {
  return createApiClient({ ...options, maximumResponseBytes: 16 * 1024 * 1024 });
}
