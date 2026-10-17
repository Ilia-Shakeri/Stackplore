import type {
  ApiClient,
  ProductAlertPage,
  AlertRuleRecord,
  AlertRuleInput,
} from '@stackplore/api-client';
import type { ProductApi, SessionActionRunner } from './product-api-types';
import {
  pageQuery,
  isRecord,
  hasExactKeys,
  resourceIdPattern,
  tokenPattern,
  namespacedCodePattern,
  eventSeverities,
  isOneOf,
  isTimestamp,
  isSafeRangeInteger,
  parsePlainPage,
  sourceQuery,
} from './product-api-common';

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
  (value.environment_id === undefined ||
    (typeof value.environment_id === 'string' && tokenPattern.test(value.environment_id))) &&
  ['pending', 'firing', 'acknowledged', 'resolved'].includes(String(value.state)) &&
  ['debug', 'info', 'warning', 'error', 'critical'].includes(String(value.severity)) &&
  typeof value.summary_code === 'string' &&
  value.summary_code.length <= 96 &&
  namespacedCodePattern.test(value.summary_code) &&
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

export function createAlertApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): Pick<
  ProductApi,
  | 'acknowledgeAlert'
  | 'createAlertRule'
  | 'deleteAlertRule'
  | 'listAlerts'
  | 'listAlertRules'
  | 'replaceAlertRule'
> {
  return {
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
        if (!isAlert(response.data) || response.data.id !== alertId)
          throw new Error('Alert response is invalid.');
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
    async listAlerts(options = {}) {
      const response = await client.request<unknown>({
        path: '/alerts',
        query: {
          ...pageQuery(options, 100),
          ...sourceQuery(options.sourceId),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isAlert, 'Alert response is invalid.');
    },
    async listAlertRules(options = {}) {
      const response = await client.request<unknown>({
        path: '/alert-rules',
        query: {
          ...pageQuery(options, 100),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isAlertRule, 'Alert rule response is invalid.');
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
        if (
          !isAlertRule(response.data) ||
          response.data.id !== ruleId ||
          response.data.source_id !== input.source_id
        )
          throw new Error('Alert rule response is invalid.');
        return response.data;
      });
    },
  };
}
