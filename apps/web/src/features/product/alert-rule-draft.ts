import type { AlertRuleInput, AlertRuleRecord } from '@stackplore/api-client';
import { isDisplayText } from './product-api-common';

export type RuleKind = AlertRuleInput['kind'];
export type Severity = AlertRuleInput['severity'];

export interface RuleDraft {
  comparison: NonNullable<AlertRuleInput['comparison_operator']>;
  displayName: string;
  enabled: boolean;
  environmentId: string;
  evaluationSeconds: string;
  kind: RuleKind;
  minimumSeverity: Severity;
  pendingSeconds: string;
  resourceId: string;
  severity: Severity;
  sourceId: string;
  thresholdCount: string;
  thresholdValue: string;
  windowSeconds: string;
}

export const kinds: readonly RuleKind[] = [
  'resource_unavailable',
  'resource_unhealthy',
  'restart_threshold',
  'cpu_threshold',
  'memory_threshold',
  'workload_unavailable_replicas',
  'event_severity_match',
  'provider_connection_lost',
];
export const severities: readonly Severity[] = ['debug', 'info', 'warning', 'error', 'critical'];
export const comparisons: readonly NonNullable<AlertRuleInput['comparison_operator']>[] = [
  'greater_than',
  'greater_than_or_equal',
  'less_than',
  'less_than_or_equal',
  'equal',
];

export interface RuleDefaults {
  readonly windowSeconds: number;
  readonly pendingSeconds: number;
}

export const emptyDraft = (defaults?: RuleDefaults): RuleDraft => ({
  comparison: 'greater_than_or_equal',
  displayName: '',
  enabled: true,
  environmentId: '',
  evaluationSeconds: '30',
  kind: 'resource_unavailable',
  minimumSeverity: 'warning',
  pendingSeconds: defaults === undefined ? '' : String(defaults.pendingSeconds),
  resourceId: '',
  severity: 'warning',
  sourceId: '',
  thresholdCount: '1',
  thresholdValue: '80',
  windowSeconds: defaults === undefined ? '' : String(defaults.windowSeconds),
});

export const isHealthKind = (kind: RuleKind) =>
  kind === 'resource_unavailable' || kind === 'resource_unhealthy';
export const isMetricKind = (kind: RuleKind) =>
  kind === 'cpu_threshold' || kind === 'memory_threshold';
export const isCountKind = (kind: RuleKind) =>
  kind === 'restart_threshold' || kind === 'workload_unavailable_replicas';
export const needsResource = (kind: RuleKind) =>
  isHealthKind(kind) || isMetricKind(kind) || isCountKind(kind);

function positiveInteger(value: string, minimum: number, maximum: number) {
  if (value.trim() === '') return null;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= minimum && parsed <= maximum ? parsed : null;
}

export function draftInput(draft: RuleDraft): AlertRuleInput | null {
  const evaluation = positiveInteger(draft.evaluationSeconds, 1, 86_400);
  const pending = positiveInteger(draft.pendingSeconds, 0, 86_400);
  if (
    !isDisplayText(draft.displayName.trim()) ||
    !/^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(draft.sourceId.trim()) ||
    evaluation === null ||
    pending === null
  )
    return null;
  const resource = draft.resourceId.trim();
  const environment = draft.environmentId.trim();
  if (
    draft.kind !== 'provider_connection_lost' &&
    environment !== '' &&
    !/^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(environment)
  )
    return null;
  if (
    (needsResource(draft.kind) || (draft.kind === 'event_severity_match' && resource !== '')) &&
    !/^r1_[A-Za-z0-9_-]{43}$/u.test(resource)
  )
    return null;
  const input: AlertRuleInput = {
    display_name: draft.displayName.trim(),
    enabled: draft.enabled,
    evaluation_interval_seconds: evaluation,
    kind: draft.kind,
    pending_for_seconds: pending,
    severity: draft.severity,
    source_id: draft.sourceId.trim(),
    ...(draft.kind === 'provider_connection_lost' || environment === ''
      ? {}
      : { environment_id: environment }),
    ...(needsResource(draft.kind) || (draft.kind === 'event_severity_match' && resource !== '')
      ? { resource_id: resource }
      : {}),
  };
  if (isMetricKind(draft.kind)) {
    const threshold = Number(draft.thresholdValue);
    const window = positiveInteger(draft.windowSeconds, 1, 86_400);
    if (
      !Number.isFinite(threshold) ||
      draft.thresholdValue.trim() === '' ||
      threshold < 0 ||
      threshold > Number.MAX_SAFE_INTEGER ||
      window === null
    )
      return null;
    return {
      ...input,
      comparison_operator: draft.comparison,
      threshold_value: threshold,
      window_seconds: window,
    };
  }
  if (isCountKind(draft.kind)) {
    const threshold = positiveInteger(draft.thresholdCount, 1, Number.MAX_SAFE_INTEGER);
    const window = positiveInteger(draft.windowSeconds, 1, 86_400);
    if (threshold === null || window === null) return null;
    return {
      ...input,
      comparison_operator: draft.comparison,
      threshold_count: threshold,
      window_seconds: window,
    };
  }
  if (draft.kind === 'event_severity_match')
    return { ...input, minimum_event_severity: draft.minimumSeverity };
  return input;
}

export function recordInput(rule: AlertRuleRecord): AlertRuleInput {
  return {
    display_name: rule.display_name,
    enabled: rule.enabled,
    evaluation_interval_seconds: rule.evaluation_interval_seconds,
    kind: rule.kind,
    pending_for_seconds: rule.pending_for_seconds,
    severity: rule.severity,
    source_id: rule.source_id,
    ...(rule.environment_id === undefined ? {} : { environment_id: rule.environment_id }),
    ...(rule.resource_id === undefined ? {} : { resource_id: rule.resource_id }),
    ...(rule.comparison_operator === undefined
      ? {}
      : { comparison_operator: rule.comparison_operator }),
    ...(rule.threshold_value === undefined ? {} : { threshold_value: rule.threshold_value }),
    ...(rule.threshold_count === undefined ? {} : { threshold_count: rule.threshold_count }),
    ...(rule.window_seconds === undefined ? {} : { window_seconds: rule.window_seconds }),
    ...(rule.minimum_event_severity === undefined
      ? {}
      : { minimum_event_severity: rule.minimum_event_severity }),
  };
}

export function recordDraft(rule: AlertRuleRecord): RuleDraft {
  return {
    comparison: rule.comparison_operator ?? 'greater_than_or_equal',
    displayName: rule.display_name,
    enabled: rule.enabled,
    environmentId: rule.environment_id ?? '',
    evaluationSeconds: String(rule.evaluation_interval_seconds),
    kind: rule.kind,
    minimumSeverity: rule.minimum_event_severity ?? 'warning',
    pendingSeconds: String(rule.pending_for_seconds),
    resourceId: rule.resource_id ?? '',
    severity: rule.severity,
    sourceId: rule.source_id,
    thresholdCount: String(rule.threshold_count ?? 1),
    thresholdValue: String(rule.threshold_value ?? 80),
    windowSeconds: rule.window_seconds === undefined ? '' : String(rule.window_seconds),
  };
}
