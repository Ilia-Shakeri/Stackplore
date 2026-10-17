import type {
  ApiClient,
  ProductNotificationPage,
  AuditEntry,
  SystemReadiness,
  NotificationSettingsStatus,
  NotificationTestResponse,
} from '@stackplore/api-client';
import type { ProductApi, SessionActionRunner } from './product-api-types';
import {
  pageQuery,
  isRecord,
  hasExactKeys,
  resourceIdPattern,
  tokenPattern,
  namespacedCodePattern,
  isTimestamp,
  parsePlainPage,
} from './product-api-common';

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

export function createActivityApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): Pick<
  ProductApi,
  | 'getNotificationSettings'
  | 'testNotification'
  | 'getSystemReadiness'
  | 'listAudit'
  | 'listNotifications'
> {
  return {
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
    async listAudit(options = {}) {
      const response = await client.request<unknown>({
        path: '/audit',
        query: {
          ...pageQuery(options, 50),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isAuditEntry, 'Audit response is invalid.');
    },
    async listNotifications(options = {}) {
      const response = await client.request<unknown>({
        path: '/notifications',
        query: {
          ...pageQuery(options, 50),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parsePlainPage(response.data, isNotification, 'Notification response is invalid.');
    },
  };
}
