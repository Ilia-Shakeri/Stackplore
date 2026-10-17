import {
  hasExactKeys,
  isDisplayText,
  isRecord,
  isSafeRangeInteger,
  isTimestamp,
} from '../product/product-api-common';
import {
  cleanupClasses,
  type ApplicationSettings,
  type ApplicationValues,
  type CleanupClass,
  type CleanupImpact,
  type CleanupPreview,
  type CleanupResult,
  type ConfigurationExport,
  type SettingsPreview,
} from './settings-types';

export const valueBounds = {
  metric_retention_hours: [1, 8760],
  health_retention_hours: [1, 8760],
  event_retention_hours: [1, 8760],
  delivery_retention_hours: [1, 8760],
  log_line_limit: [10, 200],
  default_alert_window_seconds: [60, 86400],
  default_alert_pending_seconds: [0, 3600],
} as const satisfies Record<keyof ApplicationValues, readonly [number, number]>;

export const valueNames = Object.keys(valueBounds) as (keyof ApplicationValues)[];
const integer = (value: unknown, minimum = 0, maximum = Number.MAX_SAFE_INTEGER) =>
  isSafeRangeInteger(value, minimum, maximum);
const timestamp = (value: unknown): value is string => isTimestamp(value) && value.length <= 27;
const version = (value: unknown): value is string =>
  typeof value === 'string' &&
  value.length >= 1 &&
  value.length <= 64 &&
  !/[\p{Cc}\p{Cf}\p{Zl}\p{Zp}]/u.test(value);
const proof = (value: unknown): value is string =>
  typeof value === 'string' && /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u.test(value);
export const isCleanupClass = (value: unknown): value is CleanupClass =>
  cleanupClasses.some((name) => name === value);
const scope = (value: unknown): value is CleanupClass[] =>
  Array.isArray(value) &&
  value.length >= 1 &&
  value.length <= 4 &&
  value.every(isCleanupClass) &&
  new Set(value).size === value.length;

export const isApplicationValues = (value: unknown): value is ApplicationValues =>
  isRecord(value) &&
  hasExactKeys(value, valueNames) &&
  valueNames.every((key) => integer(value[key], valueBounds[key][0], valueBounds[key][1]));
export const sameValues = (left: ApplicationValues, right: ApplicationValues) =>
  valueNames.every((key) => left[key] === right[key]);

export function isApplicationSettings(value: unknown): value is ApplicationSettings {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, [
      'version',
      'updated_at',
      'values',
      'mutation_mode',
      'audit_policy',
      'application_version',
    ]) ||
    !integer(value.version, 1) ||
    !timestamp(value.updated_at) ||
    !isApplicationValues(value.values) ||
    !version(value.application_version)
  )
    return false;
  const mode = value.mutation_mode;
  const audit = value.audit_policy;
  return (
    isRecord(mode) &&
    hasExactKeys(mode, ['enabled', 'docker_write_enabled', 'kubernetes_write_enabled']) &&
    typeof mode.enabled === 'boolean' &&
    typeof mode.docker_write_enabled === 'boolean' &&
    typeof mode.kubernetes_write_enabled === 'boolean' &&
    isRecord(audit) &&
    hasExactKeys(audit, [
      'retention_days',
      'minimum_retention_days',
      'quota_rows',
      'stored_rows',
      'quota_exceeded',
      'archive_verified_through',
    ]) &&
    integer(audit.retention_days, 90, 3650) &&
    audit.minimum_retention_days === 90 &&
    integer(audit.quota_rows, 1000, 10000000) &&
    integer(audit.stored_rows) &&
    typeof audit.quota_exceeded === 'boolean' &&
    (audit.archive_verified_through === null || timestamp(audit.archive_verified_through))
  );
}

export function isCleanupImpact(value: unknown): value is CleanupImpact {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['classes', 'maximum_rows', 'backup_required', 'expiry_policy_changed']) ||
    value.maximum_rows !== 1000 ||
    typeof value.backup_required !== 'boolean' ||
    typeof value.expiry_policy_changed !== 'boolean' ||
    !Array.isArray(value.classes) ||
    value.classes.length > 4
  )
    return false;
  const names = new Set<string>();
  return value.classes.every((item: unknown) => {
    if (
      !isRecord(item) ||
      !hasExactKeys(item, ['name', 'cutoff', 'eligible_rows', 'eligible_rows_capped']) ||
      !isCleanupClass(item.name) ||
      !timestamp(item.cutoff) ||
      !integer(item.eligible_rows, 0, 10001) ||
      typeof item.eligible_rows_capped !== 'boolean' ||
      names.has(item.name)
    )
      return false;
    if (item.eligible_rows_capped !== (item.eligible_rows === 10001)) return false;
    names.add(item.name);
    return true;
  });
}

export function isSettingsPreview(value: unknown): value is SettingsPreview {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['settings', 'requested', 'impact', 'confirmation_token', 'expires_at']) ||
    !isApplicationSettings(value.settings) ||
    !isApplicationValues(value.requested) ||
    !isCleanupImpact(value.impact) ||
    !proof(value.confirmation_token) ||
    !timestamp(value.expires_at)
  )
    return false;
  const current = value.settings.values;
  const requested = value.requested;
  const retentionClasses = [
    ['metric_retention_hours', 'metric_samples'],
    ['health_retention_hours', 'health_history'],
    ['event_retention_hours', 'infrastructure_events'],
    ['delivery_retention_hours', 'terminal_notification_deliveries'],
  ] as const;
  const changed = retentionClasses
    .filter(([key]) => requested[key] < current[key])
    .map(([, name]) => name);
  return (
    value.impact.expiry_policy_changed === changed.length > 0 &&
    value.impact.backup_required === changed.length > 0 &&
    value.impact.classes.length === changed.length &&
    value.impact.classes.every((item) => changed.some((name) => name === item.name))
  );
}

export function isCleanupPreview(value: unknown): value is CleanupPreview {
  return (
    isRecord(value) &&
    hasExactKeys(value, ['policy_version', 'impact', 'confirmation_token', 'expires_at']) &&
    integer(value.policy_version, 1) &&
    isCleanupImpact(value.impact) &&
    value.impact.backup_required &&
    value.impact.classes.length >= 1 &&
    !value.impact.expiry_policy_changed &&
    proof(value.confirmation_token) &&
    timestamp(value.expires_at)
  );
}

export function isCleanupResult(value: unknown): value is CleanupResult {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, [
      'removed_rows',
      'remaining_rows',
      'scope',
      'cutoffs',
      'policy_version',
    ]) ||
    !integer(value.removed_rows, 0, 1000) ||
    typeof value.remaining_rows !== 'boolean' ||
    !scope(value.scope) ||
    !integer(value.policy_version, 1) ||
    !Array.isArray(value.cutoffs) ||
    value.cutoffs.length !== value.scope.length
  )
    return false;
  const names = new Set<string>();
  const allowedScope = value.scope;
  return value.cutoffs.every((item: unknown) => {
    if (
      !isRecord(item) ||
      !hasExactKeys(item, ['name', 'cutoff']) ||
      !isCleanupClass(item.name) ||
      !timestamp(item.cutoff) ||
      names.has(item.name) ||
      !allowedScope.includes(item.name)
    )
      return false;
    names.add(item.name);
    return true;
  });
}

export function isConfigurationExport(value: unknown): value is ConfigurationExport {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, [
      'application_version',
      'settings',
      'sources',
      'notification_integrations',
    ]) ||
    !version(value.application_version) ||
    !isApplicationValues(value.settings) ||
    !Array.isArray(value.sources) ||
    value.sources.length > 100 ||
    !Array.isArray(value.notification_integrations) ||
    value.notification_integrations.length > 2 ||
    new TextEncoder().encode(JSON.stringify(value)).byteLength > 65536
  )
    return false;
  const kinds = new Set<string>();
  return (
    value.sources.every(
      (source: unknown) =>
        isRecord(source) &&
        hasExactKeys(source, [
          'provider',
          'name',
          'enabled',
          'profile',
          'credential_reference',
          'host',
        ]) &&
        (source.provider === 'demo' ||
          source.provider === 'docker' ||
          source.provider === 'kubernetes') &&
        isDisplayText(source.name) &&
        typeof source.enabled === 'boolean' &&
        typeof source.profile === 'string' &&
        /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(source.profile) &&
        source.credential_reference === '[masked]' &&
        source.host === '[masked]',
    ) &&
    value.notification_integrations.every((item: unknown) => {
      if (
        !isRecord(item) ||
        !hasExactKeys(item, ['kind', 'enabled', 'target', 'credential_reference']) ||
        (item.kind !== 'webhook' && item.kind !== 'matrix') ||
        kinds.has(item.kind) ||
        typeof item.enabled !== 'boolean' ||
        item.target !== '[masked]' ||
        item.credential_reference !== '[masked]'
      )
        return false;
      kinds.add(item.kind);
      return true;
    })
  );
}

export function parseSettingsResponse<T>(value: unknown, guard: (value: unknown) => value is T): T {
  if (!guard(value)) throw new Error('Settings response is invalid.');
  return value;
}
