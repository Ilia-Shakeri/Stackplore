import type { ProductRelationshipPage } from '@stackplore/api-client';
import type {
  ObservationReadOptions,
  ProductPageOptions,
  ResourceSelectionOptions,
} from './product-api-types';

export const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

export const hasExactKeys = (
  value: Record<string, unknown>,
  required: readonly string[],
  optional: readonly string[] = [],
) => {
  const allowed = new Set([...required, ...optional]);
  const keys = Object.keys(value);
  return (
    required.every((key) => Object.hasOwn(value, key)) && keys.every((key) => allowed.has(key))
  );
};

export const resourceIdPattern = /^r1_[A-Za-z0-9_-]{43}$/u;

export const cursorPattern = /^[A-Za-z0-9_-]{1,4096}$/u;

export const relationshipIdPattern = /^rel1_[A-Za-z0-9_-]{43}$/u;

export const tokenPattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;

export const namespacedCodePattern = /^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+$/u;

export const isDisplayText = (value: unknown): value is string =>
  typeof value === 'string' &&
  value.length >= 1 &&
  value.length <= 256 &&
  /^(?=\P{White_Space})(?!.*\p{White_Space}$)(?=.*[\p{L}\p{N}\p{P}\p{S}])(?:[^\p{Cc}\p{Cf}\p{Zl}\p{Zp}]|\u200C|\u200D)+$/u.test(
    value,
  );

export const eventSeverities = new Set(['debug', 'info', 'warning', 'error', 'critical']);

export const providers = new Set(['demo', 'docker', 'kubernetes']);

export const scopes = new Set(['global', 'project', 'namespace']);

export const healthStates = new Set(['healthy', 'degraded', 'unhealthy', 'unknown']);

export const availabilityStates = new Set([
  'available',
  'unavailable',
  'permission_denied',
  'unsupported',
  'stale',
]);

export const completenessStates = new Set(['complete', 'partial']);

export const isOneOf = (value: unknown, values: ReadonlySet<string>): value is string =>
  typeof value === 'string' && values.has(value);

export const isTimestamp = (value: unknown): value is string => {
  if (
    typeof value !== 'string' ||
    value.length < 20 ||
    value.length > 30 ||
    !/^(?!0000)[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z$/u.test(
      value,
    )
  )
    return false;
  const parsed = new Date(value);
  return (
    Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 19) === value.slice(0, 19)
  );
};
export const isCount = (value: unknown) =>
  typeof value === 'number' && Number.isInteger(value) && value >= 0;

export const isSafeRangeInteger = (value: unknown, minimum: number, maximum: number) =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum && value <= maximum;

export const isObservationMetadata = (value: Record<string, unknown>) =>
  isOneOf(value.availability, availabilityStates) &&
  isOneOf(value.completeness, completenessStates) &&
  isTimestamp(value.observed_at) &&
  (value.reason_code === undefined ||
    (typeof value.reason_code === 'string' &&
      value.reason_code.length <= 96 &&
      namespacedCodePattern.test(value.reason_code))) &&
  (value.availability === 'available' && value.completeness === 'complete'
    ? true
    : value.reason_code !== undefined);

export const isPage = (value: unknown): value is ProductRelationshipPage['page'] =>
  isRecord(value) &&
  Number.isInteger(value.limit) &&
  Number(value.limit) >= 1 &&
  Number(value.limit) <= 200 &&
  typeof value.has_more === 'boolean' &&
  (value.has_more
    ? hasExactKeys(value, ['limit', 'has_more', 'next_cursor']) &&
      typeof value.next_cursor === 'string' &&
      cursorPattern.test(value.next_cursor)
    : hasExactKeys(value, ['limit', 'has_more']));

export const parseObservedPage = <T>(
  value: unknown,
  itemGuard: (item: unknown) => item is T,
  message: string,
) => {
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
    !value.items.every(itemGuard) ||
    !isPage(value.page) ||
    value.items.length > value.page.limit ||
    (!['available', 'stale'].includes(String(value.availability)) && value.items.length !== 0)
  ) {
    throw new Error(message);
  }
  return value as {
    readonly availability: ProductRelationshipPage['availability'];
    readonly completeness: ProductRelationshipPage['completeness'];
    readonly observed_at: string;
    readonly reason_code?: string;
    readonly items: T[];
    readonly page: ProductRelationshipPage['page'];
  };
};

export const parsePlainPage = <T>(
  value: unknown,
  itemGuard: (item: unknown) => item is T,
  message: string,
) => {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['items', 'page']) ||
    !Array.isArray(value.items) ||
    value.items.length > 200 ||
    !value.items.every(itemGuard) ||
    !isPage(value.page) ||
    value.items.length > value.page.limit
  ) {
    throw new Error(message);
  }
  return value as { readonly items: T[]; readonly page: ProductRelationshipPage['page'] };
};

export const sourceQuery = (sourceId?: string) => {
  if (sourceId !== undefined && !tokenPattern.test(sourceId)) {
    throw new Error('Source identifier is invalid.');
  }
  return sourceId === undefined ? {} : { source_id: sourceId };
};

export const pageQuery = (options: ProductPageOptions, defaultLimit: number) => {
  if (options.cursor !== undefined && !cursorPattern.test(options.cursor)) {
    throw new Error('Page cursor is invalid.');
  }
  if (options.limit !== undefined && !isSafeRangeInteger(options.limit, 1, 200)) {
    throw new Error('Page limit is invalid.');
  }
  return {
    limit: options.limit ?? defaultLimit,
    ...(options.cursor === undefined ? {} : { cursor: options.cursor }),
  };
};

export const readQuery = (options: ResourceSelectionOptions) => {
  if (options.resourceId !== undefined && !resourceIdPattern.test(options.resourceId)) {
    throw new Error('Resource identifier is invalid.');
  }
  return {
    ...sourceQuery(options.sourceId),
    ...(options.resourceId === undefined ? {} : { resource_id: options.resourceId }),
  };
};

const timestampMicros = (value: string) =>
  BigInt(Date.parse(value)) * 1_000n +
  BigInt(value.slice(19, -1).replace('.', '').slice(3).padEnd(3, '0') || '0');

const normalizedTime = (value: string) =>
  `${value.slice(0, 19)}.${value.slice(19, -1).replace('.', '').padEnd(6, '0')}`;

export const timeQuery = (
  options: Pick<ObservationReadOptions, 'from' | 'to'>,
  maximumDays: number,
) => {
  if (options.from === undefined && options.to === undefined) return {};
  if (
    !isTimestamp(options.from) ||
    !isTimestamp(options.to) ||
    /\.[0-9]{7,}Z$/u.test(options.from) ||
    /\.[0-9]{7,}Z$/u.test(options.to) ||
    normalizedTime(options.to) <= normalizedTime(options.from) ||
    timestampMicros(options.to) - timestampMicros(options.from) >
      BigInt(maximumDays) * 86_400_000_000n ||
    Date.parse(options.to) > Date.now() + 60_000
  )
    throw new Error('Time range is invalid.');
  return { from: options.from, to: options.to };
};
