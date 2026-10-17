import type {
  ApiClient,
  ProductResource,
  ProductResourcePage,
  SourceSummary,
  ProductResourceHealth,
  ProductEnvironmentPage,
} from '@stackplore/api-client';
import type { ProductApi, ResourceReadOptions } from './product-api-types';
import {
  pageQuery,
  isRecord,
  isDisplayText,
  hasExactKeys,
  resourceIdPattern,
  cursorPattern,
  tokenPattern,
  namespacedCodePattern,
  providers,
  scopes,
  healthStates,
  availabilityStates,
  completenessStates,
  isOneOf,
  isTimestamp,
  isSafeRangeInteger,
  isObservationMetadata,
  parseObservedPage,
  sourceQuery,
} from './product-api-common';

const isResource = (value: unknown): value is ProductResource =>
  isRecord(value) &&
  hasExactKeys(
    value,
    [
      'id',
      'provider',
      'source_id',
      'environment_id',
      'kind',
      'scope_kind',
      'name',
      'synthetic',
      'observed_at',
    ],
    ['scope_name', 'health'],
  ) &&
  typeof value.id === 'string' &&
  resourceIdPattern.test(value.id) &&
  isOneOf(value.provider, providers) &&
  typeof value.source_id === 'string' &&
  tokenPattern.test(value.source_id) &&
  typeof value.environment_id === 'string' &&
  tokenPattern.test(value.environment_id) &&
  typeof value.kind === 'string' &&
  value.kind.length <= 96 &&
  namespacedCodePattern.test(value.kind) &&
  isOneOf(value.scope_kind, scopes) &&
  (value.scope_name === undefined || isDisplayText(value.scope_name)) &&
  typeof value.name === 'string' &&
  value.name.length > 0 &&
  value.name.length <= 256 &&
  typeof value.synthetic === 'boolean' &&
  isTimestamp(value.observed_at) &&
  (value.health === undefined || isOneOf(value.health, healthStates));

const parseResourcePage = (value: unknown): ProductResourcePage => {
  if (
    !isRecord(value) ||
    !hasExactKeys(
      value,
      ['availability', 'completeness', 'observed_at', 'items', 'page'],
      ['reason_code'],
    ) ||
    !isObservationMetadata(value) ||
    !Array.isArray(value.items) ||
    value.items.length > 200 ||
    !value.items.every(isResource) ||
    !isRecord(value.page)
  ) {
    throw new Error('Resource response is invalid.');
  }
  const page = value.page;
  if (
    !Number.isInteger(page.limit) ||
    Number(page.limit) < 1 ||
    Number(page.limit) > 200 ||
    value.items.length > Number(page.limit) ||
    (!['available', 'stale'].includes(String(value.availability)) && value.items.length !== 0) ||
    typeof page.has_more !== 'boolean' ||
    (page.has_more
      ? !hasExactKeys(page, ['limit', 'has_more', 'next_cursor']) ||
        typeof page.next_cursor !== 'string' ||
        !cursorPattern.test(page.next_cursor)
      : !hasExactKeys(page, ['limit', 'has_more']))
  ) {
    throw new Error('Resource page is invalid.');
  }
  return value as ProductResourcePage;
};

const isSource = (value: unknown): value is SourceSummary => {
  if (
    !isRecord(value) ||
    !hasExactKeys(
      value,
      [
        'id',
        'provider',
        'name',
        'synthetic',
        'observed_at',
        'availability',
        'completeness',
        'resource_count',
        'health',
      ],
      ['fixture_version'],
    ) ||
    !isRecord(value.health)
  )
    return false;
  const health = value.health;
  return (
    typeof value.id === 'string' &&
    tokenPattern.test(value.id) &&
    isOneOf(value.provider, providers) &&
    typeof value.name === 'string' &&
    value.name.length > 0 &&
    value.name.length <= 256 &&
    typeof value.synthetic === 'boolean' &&
    (value.fixture_version === undefined ||
      (typeof value.fixture_version === 'string' &&
        value.fixture_version.length > 0 &&
        value.fixture_version.length <= 64)) &&
    isTimestamp(value.observed_at) &&
    isOneOf(value.availability, availabilityStates) &&
    isOneOf(value.completeness, completenessStates) &&
    isSafeRangeInteger(value.resource_count, 0, 1_000_000) &&
    hasExactKeys(health, ['healthy', 'degraded', 'unhealthy', 'unknown']) &&
    ['healthy', 'degraded', 'unhealthy', 'unknown'].every((key) =>
      isSafeRangeInteger(health[key], 0, 1_000_000),
    )
  );
};

const isResourceHealth = (value: unknown): value is ProductResourceHealth =>
  isRecord(value) &&
  hasExactKeys(value, ['resource_id', 'state', 'observed_at', 'synthetic', 'reason_codes']) &&
  typeof value.resource_id === 'string' &&
  resourceIdPattern.test(value.resource_id) &&
  isOneOf(value.state, healthStates) &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean' &&
  Array.isArray(value.reason_codes) &&
  value.reason_codes.length <= 32 &&
  value.reason_codes.every(
    (code) =>
      typeof code === 'string' &&
      code.length >= 3 &&
      code.length <= 96 &&
      namespacedCodePattern.test(code),
  );

const isEnvironment = (value: unknown): value is ProductEnvironmentPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(value, [
    'id',
    'source_id',
    'provider',
    'name',
    'availability',
    'completeness',
    'observed_at',
    'synthetic',
  ]) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  typeof value.source_id === 'string' &&
  tokenPattern.test(value.source_id) &&
  isOneOf(value.provider, providers) &&
  typeof value.name === 'string' &&
  value.name.length > 0 &&
  value.name.length <= 256 &&
  value.name.length > 0 &&
  isOneOf(value.availability, availabilityStates) &&
  isOneOf(value.completeness, completenessStates) &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean';

const resourceQuery = (options: ResourceReadOptions) => {
  if (
    (options.search !== undefined && options.search.length > 200) ||
    (options.provider !== undefined && !providers.has(options.provider)) ||
    (options.environmentId !== undefined && !tokenPattern.test(options.environmentId)) ||
    (options.kind !== undefined &&
      (!namespacedCodePattern.test(options.kind) || options.kind.length > 96)) ||
    (options.health !== undefined && !healthStates.has(options.health)) ||
    (options.availability !== undefined && !availabilityStates.has(options.availability)) ||
    (options.scopeKind !== undefined && !scopes.has(options.scopeKind)) ||
    (options.scopeName !== undefined && !isDisplayText(options.scopeName)) ||
    (options.sort !== undefined && !['name', 'kind', 'observed_at'].includes(options.sort)) ||
    (options.direction !== undefined && !['asc', 'desc'].includes(options.direction))
  ) {
    throw new Error('Resource filters are invalid.');
  }
  return {
    ...pageQuery(options, 50),
    ...sourceQuery(options.sourceId),
    ...(options.search === undefined ? {} : { search: options.search }),
    ...(options.provider === undefined ? {} : { provider: options.provider }),
    ...(options.environmentId === undefined ? {} : { environment_id: options.environmentId }),
    ...(options.kind === undefined ? {} : { kind: options.kind }),
    ...(options.health === undefined ? {} : { health: options.health }),
    ...(options.availability === undefined ? {} : { availability: options.availability }),
    ...(options.scopeKind === undefined ? {} : { scope_kind: options.scopeKind }),
    ...(options.scopeName === undefined ? {} : { scope_name: options.scopeName }),
    ...(options.sort === undefined ? {} : { sort: options.sort }),
    ...(options.direction === undefined ? {} : { direction: options.direction }),
  };
};

export function createInventoryApi(
  client: ApiClient,
): Pick<
  ProductApi,
  'listEnvironments' | 'getResource' | 'getResourceHealth' | 'listResources' | 'listSources'
> {
  return {
    async listEnvironments(options = {}) {
      const response = await client.request<unknown>({
        path: '/environments',
        query: {
          ...pageQuery(options, 50),
          ...sourceQuery(options.sourceId),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      const page = parseObservedPage(
        response.data,
        isEnvironment,
        'Environment response is invalid.',
      );
      if (
        options.sourceId !== undefined &&
        page.items.some((item) => item.source_id !== options.sourceId)
      )
        throw new Error('Environment source is invalid.');
      return page;
    },
    async getResource(resourceId, signal, sourceId) {
      if (!resourceIdPattern.test(resourceId)) throw new Error('Resource identifier is invalid.');
      const response = await client.request<unknown>({
        path: `/resources/${encodeURIComponent(resourceId)}`,
        query: sourceQuery(sourceId),
        ...(signal === undefined ? {} : { signal }),
      });
      if (
        !isResource(response.data) ||
        response.data.id !== resourceId ||
        (sourceId !== undefined && response.data.source_id !== sourceId)
      )
        throw new Error('Resource response is invalid.');
      return response.data;
    },
    async getResourceHealth(resourceId, signal, sourceId) {
      if (!resourceIdPattern.test(resourceId)) throw new Error('Resource identifier is invalid.');
      const response = await client.request<unknown>({
        path: `/resources/${encodeURIComponent(resourceId)}/health`,
        query: sourceQuery(sourceId),
        ...(signal === undefined ? {} : { signal }),
      });
      if (!isResourceHealth(response.data) || response.data.resource_id !== resourceId)
        throw new Error('Resource health response is invalid.');
      return response.data;
    },
    async listResources(options = {}) {
      const response = await client.request<unknown>({
        path: '/resources',
        query: resourceQuery(options),
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      const page = parseResourcePage(response.data);
      if (
        options.sourceId !== undefined &&
        page.items.some((item) => item.source_id !== options.sourceId)
      )
        throw new Error('Resource source is invalid.');
      return page;
    },
    async listSources(signal) {
      const response = await client.request<unknown>({
        path: '/sources',
        ...(signal === undefined ? {} : { signal }),
      });
      if (
        !Array.isArray(response.data) ||
        response.data.length > 64 ||
        !response.data.every(isSource)
      ) {
        throw new Error('Source response is invalid.');
      }
      return response.data;
    },
  };
}
