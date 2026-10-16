import type {
  AlertRuleInput,
  AlertRulePage,
  AlertRuleRecord,
  ApiClient,
  AuditEntry,
  ProductAlert,
  ProductAlertPage,
  ProductAuditPage,
  ProductEnvironmentPage,
  ProductEventPage,
  ProductLogPage,
  ProductMetricHistoryResult,
  ProductMetricResult,
  ProductNotificationPage,
  NotificationSettingsStatus,
  NotificationTestResponse,
  ProductRelationshipPage,
  ProductResource,
  ProductResourceHealth,
  ProductResourcePage,
  SourceSummary,
  SystemReadiness,
} from '@stackplore/api-client';
import type { SessionProtection } from '../auth/auth-api';
import { createSourceSettingsApi, type SourceSettingsApi } from './source-settings-api';

export interface ProductApi extends SourceSettingsApi {
  acknowledgeAlert(alertId: string): Promise<ProductAlert>;
  createAlertRule(input: AlertRuleInput): Promise<AlertRuleRecord>;
  deleteAlertRule(ruleId: string, version: number): Promise<void>;
  getResource(
    resourceId: string,
    signal?: AbortSignal,
    sourceId?: string,
  ): Promise<ProductResource>;
  getResourceHealth(
    resourceId: string,
    signal?: AbortSignal,
    sourceId?: string,
  ): Promise<ProductResourceHealth>;
  getNotificationSettings(signal?: AbortSignal): Promise<NotificationSettingsStatus>;
  getSystemReadiness(signal?: AbortSignal): Promise<SystemReadiness>;
  listAlerts(options?: PageOptions): Promise<ProductAlertPage>;
  listAlertRules(options?: PageOptions): Promise<AlertRulePage>;
  listAudit(options?: PageOptions): Promise<ProductAuditPage>;
  listEnvironments(options?: PageOptions): Promise<ProductEnvironmentPage>;
  listEvents(options?: ReadOptions): Promise<ProductEventPage>;
  listLogs(options?: ReadOptions): Promise<ProductLogPage>;
  listMetricHistory(options?: MetricHistoryOptions): Promise<ProductMetricHistoryResult>;
  listMetrics(options?: ReadOptions): Promise<ProductMetricResult>;
  listNotifications(options?: PageOptions): Promise<ProductNotificationPage>;
  listResources(options?: {
    readonly search?: string;
    readonly signal?: AbortSignal;
    readonly sourceId?: string;
  }): Promise<ProductResourcePage>;
  listSources(signal?: AbortSignal): Promise<readonly SourceSummary[]>;
  listTopology(options?: ReadOptions): Promise<ProductRelationshipPage>;
  replaceAlertRule(
    ruleId: string,
    version: number,
    input: AlertRuleInput,
  ): Promise<AlertRuleRecord>;
  testNotification(
    currentPassword: string,
    adapter?: 'generic_webhook' | 'matrix',
  ): Promise<NotificationTestResponse>;
}

export interface SessionActionRunner {
  runRecentAction?<Result>(
    purpose: 'notification.test' | 'source.update',
    currentPassword: string,
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result>;
  runSessionAction<Result>(
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result>;
}

interface PageOptions {
  readonly cursor?: string;
  readonly limit?: number;
  readonly signal?: AbortSignal;
  readonly sourceId?: string;
}

interface ReadOptions {
  readonly resourceId?: string;
  readonly signal?: AbortSignal;
  readonly sourceId?: string;
}

interface MetricHistoryOptions extends ReadOptions {
  readonly minutes?: number;
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

const hasExactKeys = (
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

const resourceIdPattern = /^r1_[A-Za-z0-9_-]{43}$/u;
const cursorPattern = /^[A-Za-z0-9_-]{1,4096}$/u;
const relationshipIdPattern = /^rel1_[A-Za-z0-9_-]{43}$/u;
const tokenPattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const namespacedCodePattern = /^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+$/u;
const alertRuleKinds = new Set([
  'resource_unavailable',
  'resource_unhealthy',
  'restart_threshold',
  'cpu_threshold',
  'memory_threshold',
  'workload_unavailable_replicas',
  'event_severity_match',
  'provider_connection_lost',
]);
const comparisonOperators = new Set([
  'greater_than',
  'greater_than_or_equal',
  'less_than',
  'less_than_or_equal',
  'equal',
]);
const eventSeverities = new Set(['debug', 'info', 'warning', 'error', 'critical']);
const providers = new Set(['demo', 'docker', 'kubernetes']);
const scopes = new Set(['global', 'project', 'namespace']);
const healthStates = new Set(['healthy', 'degraded', 'unhealthy', 'unknown']);
const availabilityStates = new Set([
  'available',
  'unavailable',
  'permission_denied',
  'unsupported',
  'stale',
]);
const completenessStates = new Set(['complete', 'partial']);
const isOneOf = (value: unknown, values: ReadonlySet<string>): value is string =>
  typeof value === 'string' && values.has(value);
const isTimestamp = (value: unknown): value is string =>
  typeof value === 'string' &&
  value.length >= 20 &&
  value.length <= 30 &&
  /^(?!0000)[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z$/u.test(
    value,
  ) &&
  Number.isFinite(Date.parse(value));
const isCount = (value: unknown) =>
  typeof value === 'number' && Number.isInteger(value) && value >= 0;
const isSafeRangeInteger = (value: unknown, minimum: number, maximum: number) =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum && value <= maximum;

const isObservationMetadata = (value: Record<string, unknown>) =>
  isOneOf(value.availability, availabilityStates) &&
  isOneOf(value.completeness, completenessStates) &&
  isTimestamp(value.observed_at) &&
  (value.reason_code === undefined ||
    (typeof value.reason_code === 'string' && value.reason_code.length > 0)) &&
  (value.availability === 'available' && value.completeness === 'complete'
    ? true
    : value.reason_code !== undefined);

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
  value.source_id.length > 0 &&
  typeof value.environment_id === 'string' &&
  value.environment_id.length > 0 &&
  typeof value.kind === 'string' &&
  value.kind.length > 0 &&
  isOneOf(value.scope_kind, scopes) &&
  (value.scope_name === undefined || typeof value.scope_name === 'string') &&
  typeof value.name === 'string' &&
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
    value.id.length > 0 &&
    isOneOf(value.provider, providers) &&
    typeof value.name === 'string' &&
    typeof value.synthetic === 'boolean' &&
    (value.fixture_version === undefined || typeof value.fixture_version === 'string') &&
    isTimestamp(value.observed_at) &&
    isOneOf(value.availability, availabilityStates) &&
    isOneOf(value.completeness, completenessStates) &&
    isCount(value.resource_count) &&
    hasExactKeys(health, ['healthy', 'degraded', 'unhealthy', 'unknown']) &&
    ['healthy', 'degraded', 'unhealthy', 'unknown'].every((key) => isCount(health[key]))
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

const isPage = (value: unknown): value is ProductRelationshipPage['page'] =>
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

const parseObservedPage = <T>(
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
    value.items.length > value.page.limit
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

const parsePlainPage = <T>(
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
  value.confidence <= 1;

const isLog = (value: unknown): value is ProductLogPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(value, [
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
  ]) &&
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
    !value.items.every(isMetric)
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
  (value.environment_id === undefined || typeof value.environment_id === 'string') &&
  typeof value.kind === 'string' &&
  typeof value.reason_code === 'string' &&
  ['debug', 'info', 'warning', 'error', 'critical'].includes(String(value.severity)) &&
  typeof value.summary === 'string' &&
  value.summary.length <= 8192 &&
  isTimestamp(value.occurred_at) &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean';

const isAlert = (value: unknown): value is ProductAlertPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(
    value,
    ['id', 'rule_id', 'state', 'severity', 'summary_code', 'started_at', 'updated_at', 'synthetic'],
    ['resource_id', 'environment_id', 'acknowledged_at', 'resolved_at'],
  ) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  typeof value.rule_id === 'string' &&
  tokenPattern.test(value.rule_id) &&
  (value.resource_id === undefined ||
    (typeof value.resource_id === 'string' && resourceIdPattern.test(value.resource_id))) &&
  (value.environment_id === undefined || typeof value.environment_id === 'string') &&
  ['pending', 'firing', 'acknowledged', 'resolved'].includes(String(value.state)) &&
  ['debug', 'info', 'warning', 'error', 'critical'].includes(String(value.severity)) &&
  typeof value.summary_code === 'string' &&
  value.summary_code.length > 0 &&
  isTimestamp(value.started_at) &&
  isTimestamp(value.updated_at) &&
  (value.acknowledged_at === undefined || isTimestamp(value.acknowledged_at)) &&
  (value.resolved_at === undefined || isTimestamp(value.resolved_at)) &&
  (value.state === 'acknowledged'
    ? value.acknowledged_at !== undefined && value.resolved_at === undefined
    : true) &&
  (value.state === 'resolved'
    ? value.resolved_at !== undefined
    : value.resolved_at === undefined) &&
  (value.state === 'pending' || value.state === 'firing'
    ? value.acknowledged_at === undefined
    : true) &&
  typeof value.synthetic === 'boolean';

const isAlertRule = (value: unknown): value is AlertRuleRecord =>
  isRecord(value) &&
  hasExactKeys(
    value,
    [
      'id',
      'source_id',
      'display_name',
      'kind',
      'enabled',
      'severity',
      'evaluation_interval_seconds',
      'pending_for_seconds',
      'created_at',
      'updated_at',
      'version',
    ],
    [
      'environment_id',
      'resource_id',
      'comparison_operator',
      'threshold_value',
      'threshold_count',
      'window_seconds',
      'minimum_event_severity',
    ],
  ) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  typeof value.source_id === 'string' &&
  tokenPattern.test(value.source_id) &&
  (value.environment_id === undefined ||
    (typeof value.environment_id === 'string' && tokenPattern.test(value.environment_id))) &&
  (value.resource_id === undefined ||
    (typeof value.resource_id === 'string' && resourceIdPattern.test(value.resource_id))) &&
  typeof value.display_name === 'string' &&
  value.display_name.length >= 1 &&
  value.display_name.length <= 256 &&
  isOneOf(value.kind, alertRuleKinds) &&
  typeof value.enabled === 'boolean' &&
  isOneOf(value.severity, eventSeverities) &&
  (value.comparison_operator === undefined ||
    isOneOf(value.comparison_operator, comparisonOperators)) &&
  (value.threshold_value === undefined ||
    (typeof value.threshold_value === 'number' &&
      Number.isFinite(value.threshold_value) &&
      value.threshold_value >= 0 &&
      value.threshold_value <= Number.MAX_SAFE_INTEGER)) &&
  (value.threshold_count === undefined ||
    isSafeRangeInteger(value.threshold_count, 1, Number.MAX_SAFE_INTEGER)) &&
  (value.window_seconds === undefined || isSafeRangeInteger(value.window_seconds, 1, 86_400)) &&
  (value.minimum_event_severity === undefined ||
    isOneOf(value.minimum_event_severity, eventSeverities)) &&
  isSafeRangeInteger(value.evaluation_interval_seconds, 1, 86_400) &&
  isSafeRangeInteger(value.pending_for_seconds, 0, 86_400) &&
  isTimestamp(value.created_at) &&
  isTimestamp(value.updated_at) &&
  isSafeRangeInteger(value.version, 1, Number.MAX_SAFE_INTEGER) &&
  hasAlertRuleShape(value);

const hasAlertRuleShape = (value: Record<string, unknown>) => {
  const noThreshold =
    value.comparison_operator === undefined &&
    value.threshold_value === undefined &&
    value.threshold_count === undefined &&
    value.window_seconds === undefined &&
    value.minimum_event_severity === undefined;
  if (value.kind === 'resource_unavailable' || value.kind === 'resource_unhealthy') {
    return value.resource_id !== undefined && noThreshold;
  }
  if (value.kind === 'cpu_threshold' || value.kind === 'memory_threshold') {
    return (
      value.resource_id !== undefined &&
      value.comparison_operator !== undefined &&
      value.threshold_value !== undefined &&
      value.threshold_count === undefined &&
      value.window_seconds !== undefined &&
      value.minimum_event_severity === undefined
    );
  }
  if (value.kind === 'restart_threshold' || value.kind === 'workload_unavailable_replicas') {
    return (
      value.resource_id !== undefined &&
      value.comparison_operator !== undefined &&
      value.threshold_value === undefined &&
      value.threshold_count !== undefined &&
      value.window_seconds !== undefined &&
      value.minimum_event_severity === undefined
    );
  }
  if (value.kind === 'event_severity_match') {
    return (
      value.comparison_operator === undefined &&
      value.threshold_value === undefined &&
      value.threshold_count === undefined &&
      value.window_seconds === undefined &&
      value.minimum_event_severity !== undefined
    );
  }
  return value.environment_id === undefined && value.resource_id === undefined && noThreshold;
};

const isNotification = (value: unknown): value is ProductNotificationPage['items'][number] =>
  isRecord(value) &&
  hasExactKeys(
    value,
    ['id', 'kind', 'severity', 'summary_code', 'created_at', 'synthetic'],
    ['source_id', 'alert_id', 'resource_id'],
  ) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  (value.source_id === undefined ||
    (typeof value.source_id === 'string' && tokenPattern.test(value.source_id))) &&
  (value.alert_id === undefined ||
    (typeof value.alert_id === 'string' && tokenPattern.test(value.alert_id))) &&
  (value.resource_id === undefined ||
    (typeof value.resource_id === 'string' && resourceIdPattern.test(value.resource_id))) &&
  typeof value.kind === 'string' &&
  namespacedCodePattern.test(value.kind) &&
  ['debug', 'info', 'warning', 'error', 'critical'].includes(String(value.severity)) &&
  typeof value.summary_code === 'string' &&
  namespacedCodePattern.test(value.summary_code) &&
  isTimestamp(value.created_at) &&
  typeof value.synthetic === 'boolean';

const isAuditActor = (value: unknown): value is AuditEntry['actor'] => {
  if (!isRecord(value) || typeof value.kind !== 'string') return false;
  if (value.kind === 'system') return hasExactKeys(value, ['kind']);
  if (value.kind === 'user') {
    return (
      hasExactKeys(value, ['kind', 'id']) &&
      typeof value.id === 'string' &&
      tokenPattern.test(value.id)
    );
  }
  return (
    value.kind === 'operator' &&
    hasExactKeys(value, ['kind'], ['id']) &&
    (value.id === undefined || (typeof value.id === 'string' && tokenPattern.test(value.id)))
  );
};

const isAuditTarget = (value: unknown): value is AuditEntry['target'] =>
  isRecord(value) &&
  hasExactKeys(value, ['kind'], ['id']) &&
  typeof value.kind === 'string' &&
  namespacedCodePattern.test(value.kind) &&
  (value.id === undefined || (typeof value.id === 'string' && tokenPattern.test(value.id)));

const isAuditEntry = (value: unknown): value is AuditEntry =>
  isRecord(value) &&
  hasExactKeys(value, [
    'id',
    'occurred_at',
    'actor',
    'action',
    'target',
    'outcome',
    'request_id',
    'summary_code',
  ]) &&
  typeof value.id === 'string' &&
  tokenPattern.test(value.id) &&
  isTimestamp(value.occurred_at) &&
  isAuditActor(value.actor) &&
  typeof value.action === 'string' &&
  namespacedCodePattern.test(value.action) &&
  isAuditTarget(value.target) &&
  ['intent', 'success', 'failure', 'denied', 'uncertain'].includes(String(value.outcome)) &&
  typeof value.request_id === 'string' &&
  tokenPattern.test(value.request_id) &&
  typeof value.summary_code === 'string' &&
  namespacedCodePattern.test(value.summary_code);

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
  isOneOf(value.availability, availabilityStates) &&
  isOneOf(value.completeness, completenessStates) &&
  isTimestamp(value.observed_at) &&
  typeof value.synthetic === 'boolean';

const isSystemReadiness = (value: unknown): value is SystemReadiness => {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['state', 'checks']) ||
    !['ready', 'not_ready'].includes(String(value.state)) ||
    !Array.isArray(value.checks) ||
    value.checks.length !== 3
  )
    return false;
  const checks: unknown[] = value.checks;
  return ['configuration', 'database', 'service'].every((name, index) => {
    const check = checks[index];
    return (
      isRecord(check) &&
      hasExactKeys(check, ['name', 'state']) &&
      check.name === name &&
      ['ready', 'not_ready'].includes(String(check.state))
    );
  });
};

const isNotificationSettings = (value: unknown): value is NotificationSettingsStatus => {
  if (
    !isRecord(value) ||
    value.adapter !== 'generic_webhook' ||
    value.configuration_source !== 'operator_environment' ||
    value.delivery_semantics !== 'at_least_once' ||
    typeof value.enabled !== 'boolean' ||
    !isMatrixNotificationSettings(value.matrix)
  )
    return false;
  if (value.enabled) {
    return (
      hasExactKeys(value, [
        'adapter',
        'enabled',
        'destination_reference_id',
        'configuration_source',
        'delivery_semantics',
        'matrix',
      ]) &&
      typeof value.destination_reference_id === 'string' &&
      tokenPattern.test(value.destination_reference_id)
    );
  }
  return hasExactKeys(value, [
    'adapter',
    'enabled',
    'configuration_source',
    'delivery_semantics',
    'matrix',
  ]);
};

const isMatrixNotificationSettings = (value: unknown): boolean => {
  if (!isRecord(value) || value.adapter !== 'matrix' || typeof value.enabled !== 'boolean')
    return false;
  return value.enabled
    ? hasExactKeys(value, ['adapter', 'enabled', 'destination_reference_id']) &&
        typeof value.destination_reference_id === 'string' &&
        tokenPattern.test(value.destination_reference_id)
    : hasExactKeys(value, ['adapter', 'enabled']);
};

const isNotificationTestResponse = (value: unknown): value is NotificationTestResponse =>
  isRecord(value) &&
  hasExactKeys(value, ['notification_id', 'destination_reference_id', 'state']) &&
  typeof value.notification_id === 'string' &&
  tokenPattern.test(value.notification_id) &&
  typeof value.destination_reference_id === 'string' &&
  tokenPattern.test(value.destination_reference_id) &&
  value.state === 'queued';

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

const sourceQuery = (sourceId?: string) => {
  if (sourceId !== undefined && !tokenPattern.test(sourceId)) {
    throw new Error('Source identifier is invalid.');
  }
  return sourceId === undefined ? {} : { source_id: sourceId };
};

const readQuery = (options: ReadOptions) => ({
  ...sourceQuery(options.sourceId),
  ...(options.resourceId === undefined ? {} : { resource_id: options.resourceId }),
});

export function createProductApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): ProductApi {
  return {
    ...createSourceSettingsApi(client, sessionActions),
    async acknowledgeAlert(alertId) {
      if (!tokenPattern.test(alertId) || sessionActions === undefined) {
        throw new Error('Alert acknowledgement is unavailable.');
      }
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown>({
          method: 'POST',
          path: `/alerts/${encodeURIComponent(alertId)}/acknowledge`,
          protection,
        });
        if (!isAlert(response.data)) throw new Error('Alert response is invalid.');
        return response.data;
      });
    },
    async createAlertRule(input) {
      if (sessionActions === undefined) throw new Error('Alert rule management is unavailable.');
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown, AlertRuleInput>({
          body: input,
          method: 'POST',
          path: '/alert-rules',
          protection,
        });
        if (!isAlertRule(response.data)) throw new Error('Alert rule response is invalid.');
        return response.data;
      });
    },
    async deleteAlertRule(ruleId, version) {
      if (
        !tokenPattern.test(ruleId) ||
        !Number.isSafeInteger(version) ||
        version < 1 ||
        sessionActions === undefined
      ) {
        throw new Error('Alert rule management is unavailable.');
      }
      await sessionActions.runSessionAction(async (protection) => {
        await client.request<unknown>({
          headers: { 'If-Match': `"${String(version)}"` },
          method: 'DELETE',
          path: `/alert-rules/${encodeURIComponent(ruleId)}`,
          protection,
        });
      });
    },
    async getNotificationSettings(signal) {
      const response = await client.request<unknown>({
        path: '/settings/notifications',
        ...(signal === undefined ? {} : { signal }),
      });
      if (!isNotificationSettings(response.data)) {
        throw new Error('Notification settings response is invalid.');
      }
      return response.data;
    },
    async testNotification(currentPassword, adapter = 'generic_webhook') {
      if (sessionActions?.runRecentAction === undefined) {
        throw new Error('Notification test delivery is unavailable.');
      }
      return sessionActions.runRecentAction(
        'notification.test',
        currentPassword,
        async (protection) => {
          const response = await client.request<unknown>({
            method: 'POST',
            path:
              adapter === 'matrix'
                ? '/settings/notifications/test?adapter=matrix'
                : '/settings/notifications/test',
            protection,
          });
          if (!isNotificationTestResponse(response.data)) {
            throw new Error('Notification test response is invalid.');
          }
          return response.data;
        },
      );
    },
    async getSystemReadiness(signal) {
      const response = await client.request<unknown>({
        path: '/system/readiness',
        ...(signal === undefined ? {} : { signal }),
      });
      if (!isSystemReadiness(response.data)) throw new Error('Readiness response is invalid.');
      return response.data;
    },
    async listAlerts(options = {}) {
      const response = await client.request<unknown>({
        path: '/alerts',
        query: {
          limit: options.limit ?? 100,
          ...sourceQuery(options.sourceId),
          ...(options.cursor === undefined ? {} : { cursor: options.cursor }),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isAlert, 'Alert response is invalid.');
    },
    async listAlertRules(options = {}) {
      const response = await client.request<unknown>({
        path: '/alert-rules',
        query: {
          limit: options.limit ?? 100,
          ...(options.cursor === undefined ? {} : { cursor: options.cursor }),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isAlertRule, 'Alert rule response is invalid.');
    },
    async listAudit(options = {}) {
      const response = await client.request<unknown>({
        path: '/audit',
        query: {
          limit: options.limit ?? 50,
          ...(options.cursor === undefined ? {} : { cursor: options.cursor }),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isAuditEntry, 'Audit response is invalid.');
    },
    async listNotifications(options = {}) {
      const response = await client.request<unknown>({
        path: '/notifications',
        query: {
          limit: options.limit ?? 50,
          ...(options.cursor === undefined ? {} : { cursor: options.cursor }),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isNotification, 'Notification response is invalid.');
    },
    async listEnvironments(options = {}) {
      const response = await client.request<unknown>({
        path: '/environments',
        query: {
          limit: options.limit ?? 50,
          ...sourceQuery(options.sourceId),
          ...(options.cursor === undefined ? {} : { cursor: options.cursor }),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isEnvironment, 'Environment response is invalid.');
    },
    async getResource(resourceId, signal, sourceId) {
      if (!resourceIdPattern.test(resourceId)) throw new Error('Resource identifier is invalid.');
      const response = await client.request<unknown>({
        path: `/resources/${encodeURIComponent(resourceId)}`,
        query: sourceQuery(sourceId),
        ...(signal === undefined ? {} : { signal }),
      });
      if (!isResource(response.data)) throw new Error('Resource response is invalid.');
      return response.data;
    },
    async getResourceHealth(resourceId, signal, sourceId) {
      if (!resourceIdPattern.test(resourceId)) throw new Error('Resource identifier is invalid.');
      const response = await client.request<unknown>({
        path: `/resources/${encodeURIComponent(resourceId)}/health`,
        query: sourceQuery(sourceId),
        ...(signal === undefined ? {} : { signal }),
      });
      if (!isResourceHealth(response.data)) throw new Error('Resource health response is invalid.');
      return response.data;
    },
    async listEvents(options = {}) {
      const response = await client.request<unknown>({
        path: '/events',
        query: { limit: 100, ...readQuery(options) },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isEvent, 'Event response is invalid.');
    },
    async listLogs(options = {}) {
      const response = await client.request<unknown>({
        path: '/logs',
        query: { limit: 100, ...readQuery(options) },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isLog, 'Log response is invalid.');
    },
    async listMetricHistory(options = {}) {
      const response = await client.request<unknown>({
        path: '/metrics/history',
        query: {
          limit: 200,
          minutes: options.minutes ?? 60,
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
        !response.data.items.every(isMetric)
      ) {
        throw new Error('Metric response is invalid.');
      }
      return response.data as ProductMetricResult;
    },
    async listResources(options = {}) {
      const response = await client.request<unknown>({
        path: '/resources',
        query: {
          limit: 50,
          ...sourceQuery(options.sourceId),
          ...(options.search === undefined ? {} : { search: options.search }),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseResourcePage(response.data);
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
    async listTopology(options = {}) {
      const response = await client.request<unknown>({
        path: '/topology',
        query: { limit: 100, ...readQuery(options) },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseObservedPage(response.data, isRelationship, 'Topology response is invalid.');
    },
    async replaceAlertRule(ruleId, version, input) {
      if (
        !tokenPattern.test(ruleId) ||
        !Number.isSafeInteger(version) ||
        version < 1 ||
        sessionActions === undefined
      ) {
        throw new Error('Alert rule management is unavailable.');
      }
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown, AlertRuleInput>({
          body: input,
          headers: { 'If-Match': `"${String(version)}"` },
          method: 'PUT',
          path: `/alert-rules/${encodeURIComponent(ruleId)}`,
          protection,
        });
        if (!isAlertRule(response.data)) throw new Error('Alert rule response is invalid.');
        return response.data;
      });
    },
  };
}
