import type {
  ApiClient,
  ProductRelationshipPage,
  ProductLogPage,
  ProductMetricResult,
  ProductMetricHistoryResult,
  ProductEventPage,
  RelationshipEvidence,
} from '@stackplore/api-client';
import type { ProductApi, LogReadOptions } from './product-api-types';
import {
  pageQuery,
  timeQuery,
  isRecord,
  isDisplayText,
  hasExactKeys,
  resourceIdPattern,
  relationshipIdPattern,
  tokenPattern,
  namespacedCodePattern,
  providers,
  isOneOf,
  isTimestamp,
  isSafeRangeInteger,
  isObservationMetadata,
  parseObservedPage,
  readQuery,
} from './product-api-common';

const isRelationshipEvidence = (value: unknown): value is RelationshipEvidence =>
  isRecord(value) &&
  hasExactKeys(value, [
    'id',
    'provider',
    'source_id',
    'provenance',
    'certainty',
    'confidence',
    'observed_at',
    'synthetic',
  ]) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  isOneOf(value.provider, providers) &&
  typeof value.source_id === 'string' &&
  tokenPattern.test(value.source_id) &&
  [
    'ownership',
    'selector',
    'route',
    'annotation',
    'declared_dependency',
    'storage_reference',
    'network_membership',
    'endpoint',
  ].includes(String(value.provenance)) &&
  ['explicit', 'inferred'].includes(String(value.certainty)) &&
  typeof value.confidence === 'number' &&
  Number.isFinite(value.confidence) &&
  value.confidence >= 0 &&
  value.confidence <= 1 &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean';

const isRelationship = (value: unknown): value is ProductRelationshipPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(value, [
    'id',
    'from',
    'to',
    'kind',
    'direction',
    'observed_at',
    'synthetic',
    'confidence',
    'evidence',
  ]) &&
  typeof value.id === 'string' &&
  relationshipIdPattern.test(value.id) &&
  typeof value.from === 'string' &&
  resourceIdPattern.test(value.from) &&
  typeof value.to === 'string' &&
  resourceIdPattern.test(value.to) &&
  ['owns', 'selects', 'routes_to', 'depends_on', 'mounts', 'references', 'member_of'].includes(
    String(value.kind),
  ) &&
  ['source_to_target', 'bidirectional', 'undirected'].includes(String(value.direction)) &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean' &&
  typeof value.confidence === 'number' &&
  Number.isFinite(value.confidence) &&
  value.confidence >= 0 &&
  value.confidence <= 1 &&
  Array.isArray(value.evidence) &&
  value.evidence.length >= 1 &&
  value.evidence.length <= 32 &&
  value.evidence.every(isRelationshipEvidence) &&
  new Set(value.evidence.map((item: RelationshipEvidence) => item.id)).size ===
    value.evidence.length;

const isLog = (value: unknown): value is ProductLogPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(
    value,
    [
      'id',
      'resource_id',
      'stream',
      'sequence',
      'severity',
      'timestamp',
      'message',
      'redacted',
      'truncated',
      'synthetic',
    ],
    ['container'],
  ) &&
  (value.container === undefined || isDisplayText(value.container)) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  typeof value.resource_id === 'string' &&
  resourceIdPattern.test(value.resource_id) &&
  ['stdout', 'stderr', 'system', 'unknown'].includes(String(value.stream)) &&
  Number.isSafeInteger(value.sequence) &&
  Number(value.sequence) >= 0 &&
  ['trace', 'debug', 'info', 'warning', 'error', 'critical', 'unknown'].includes(
    String(value.severity),
  ) &&
  isTimestamp(value.timestamp) &&
  typeof value.message === 'string' &&
  value.message.length <= 65_536 &&
  typeof value.redacted === 'boolean' &&
  typeof value.truncated === 'boolean' &&
  typeof value.synthetic === 'boolean';

const isMetric = (value: unknown): value is ProductMetricResult['items'][number] =>
  isRecord(value) &&
  hasExactKeys(value, [
    'id',
    'resource_id',
    'name',
    'unit',
    'value',
    'sampled_at',
    'interval_seconds',
    'synthetic',
  ]) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  typeof value.resource_id === 'string' &&
  resourceIdPattern.test(value.resource_id) &&
  typeof value.name === 'string' &&
  Object.hasOwn(metricUnits, value.name) &&
  value.unit === metricUnits[value.name] &&
  typeof value.value === 'number' &&
  Number.isFinite(value.value) &&
  value.value >= 0 &&
  value.value <= Number.MAX_SAFE_INTEGER &&
  (value.unit !== 'count' || Number.isSafeInteger(value.value)) &&
  isTimestamp(value.sampled_at) &&
  Number.isInteger(value.interval_seconds) &&
  Number(value.interval_seconds) >= 0 &&
  Number(value.interval_seconds) <= 86_400 &&
  typeof value.synthetic === 'boolean';

const parseMetricHistory = (value: unknown): ProductMetricHistoryResult => {
  if (
    !isRecord(value) ||
    !hasExactKeys(
      value,
      ['availability', 'observed_at', 'completeness', 'items'],
      ['reason_code'],
    ) ||
    !isObservationMetadata(value) ||
    !Array.isArray(value.items) ||
    value.items.length > 200 ||
    !value.items.every(isMetric) ||
    (!['available', 'stale'].includes(String(value.availability)) && value.items.length !== 0)
  ) {
    throw new Error('Metric history response is invalid.');
  }
  return value as ProductMetricHistoryResult;
};

const isEvent = (value: unknown): value is ProductEventPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(
    value,
    ['id', 'kind', 'severity', 'reason_code', 'summary', 'occurred_at', 'observed_at', 'synthetic'],
    ['resource_id', 'environment_id'],
  ) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  (value.resource_id === undefined ||
    (typeof value.resource_id === 'string' && resourceIdPattern.test(value.resource_id))) &&
  (value.environment_id === undefined ||
    (typeof value.environment_id === 'string' && tokenPattern.test(value.environment_id))) &&
  typeof value.kind === 'string' &&
  value.kind.length <= 96 &&
  namespacedCodePattern.test(value.kind) &&
  typeof value.reason_code === 'string' &&
  value.reason_code.length <= 96 &&
  namespacedCodePattern.test(value.reason_code) &&
  ['debug', 'info', 'warning', 'error', 'critical'].includes(String(value.severity)) &&
  typeof value.summary === 'string' &&
  value.summary.length <= 8192 &&
  isTimestamp(value.occurred_at) &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean';

const metricUnits: Readonly<Record<string, string>> = {
  'cpu.usage.cores': 'cores',
  'memory.usage.bytes': 'bytes',
  'network.receive.bytes_total': 'bytes',
  'network.send.bytes_total': 'bytes',
  'restart.count': 'count',
  'cpu.request.cores': 'cores',
  'cpu.limit.cores': 'cores',
  'memory.request.bytes': 'bytes',
  'memory.limit.bytes': 'bytes',
};

const logQuery = (options: LogReadOptions) => {
  if (
    (options.container !== undefined &&
      !/^[A-Za-z0-9][A-Za-z0-9_.-]{0,255}$/u.test(options.container)) ||
    (options.previous !== undefined && typeof options.previous !== 'boolean') ||
    ((options.container !== undefined || options.previous !== undefined) &&
      options.resourceId === undefined)
  ) {
    throw new Error('Log selection is invalid.');
  }
  return {
    ...(options.container === undefined ? {} : { container: options.container }),
    ...(options.previous === undefined ? {} : { previous: options.previous }),
  };
};

export function createObservationApi(
  client: ApiClient,
): Pick<
  ProductApi,
  'listEvents' | 'listLogs' | 'listMetricHistory' | 'listMetrics' | 'listTopology'
> {
  return {
    async listEvents(options = {}) {
      const response = await client.request<unknown>({
        path: '/events',
        query: { ...pageQuery(options, 100), ...readQuery(options), ...timeQuery(options, 30) },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isEvent, 'Event response is invalid.');
    },
    async listLogs(options = {}) {
      const response = await client.request<unknown>({
        path: '/logs',
        query: {
          ...pageQuery(options, 100),
          ...readQuery(options),
          ...timeQuery(options, 7),
          ...logQuery(options),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isLog, 'Log response is invalid.');
    },
    async listMetricHistory(options = {}) {
      if (
        (options.minutes !== undefined && !isSafeRangeInteger(options.minutes, 5, 43_200)) ||
        (options.minutes !== undefined && (options.from !== undefined || options.to !== undefined))
      )
        throw new Error('Metric history range is invalid.');
      const response = await client.request<unknown>({
        path: '/metrics/history',
        query: {
          limit: pageQuery(options, 200).limit,
          ...(options.from === undefined && options.to === undefined
            ? { minutes: options.minutes ?? 60 }
            : timeQuery(options, 30)),
          ...readQuery(options),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseMetricHistory(response.data);
    },
    async listMetrics(options = {}) {
      const response = await client.request<unknown>({
        path: '/metrics/current',
        query: readQuery(options),
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      if (
        !isRecord(response.data) ||
        !hasExactKeys(
          response.data,
          ['availability', 'completeness', 'observed_at', 'items'],
          ['reason_code'],
        ) ||
        !isObservationMetadata(response.data) ||
        !Array.isArray(response.data.items) ||
        response.data.items.length > 200 ||
        !response.data.items.every(isMetric) ||
        (!['available', 'stale'].includes(String(response.data.availability)) &&
          response.data.items.length !== 0)
      ) {
        throw new Error('Metric response is invalid.');
      }
      return response.data as ProductMetricResult;
    },
    async listTopology(options = {}) {
      const response = await client.request<unknown>({
        path: '/topology',
        query: { ...pageQuery(options, 100), ...readQuery(options) },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isRelationship, 'Topology response is invalid.');
    },
  };
}
