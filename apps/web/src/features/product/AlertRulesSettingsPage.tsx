import type {
  AlertRuleInput,
  AlertRuleRecord,
  NotificationSettingsStatus,
} from '@stackplore/api-client';
import { useEffect, useMemo, useState, type ReactNode, type SyntheticEvent } from 'react';

import { useProductApi } from './product-context';
import { useLocale } from '../../i18n';

type RuleKind = AlertRuleInput['kind'];
type Severity = AlertRuleInput['severity'];

interface RuleDraft {
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

const kinds: readonly RuleKind[] = [
  'resource_unavailable',
  'resource_unhealthy',
  'restart_threshold',
  'cpu_threshold',
  'memory_threshold',
  'workload_unavailable_replicas',
  'event_severity_match',
  'provider_connection_lost',
];
const severities: readonly Severity[] = ['debug', 'info', 'warning', 'error', 'critical'];
const comparisons: readonly NonNullable<AlertRuleInput['comparison_operator']>[] = [
  'greater_than',
  'greater_than_or_equal',
  'less_than',
  'less_than_or_equal',
  'equal',
];

const emptyDraft = (): RuleDraft => ({
  comparison: 'greater_than_or_equal',
  displayName: '',
  enabled: true,
  environmentId: '',
  evaluationSeconds: '30',
  kind: 'resource_unavailable',
  minimumSeverity: 'warning',
  pendingSeconds: '60',
  resourceId: '',
  severity: 'warning',
  sourceId: '',
  thresholdCount: '1',
  thresholdValue: '80',
  windowSeconds: '300',
});

const isHealthKind = (kind: RuleKind) =>
  kind === 'resource_unavailable' || kind === 'resource_unhealthy';
const isMetricKind = (kind: RuleKind) => kind === 'cpu_threshold' || kind === 'memory_threshold';
const isCountKind = (kind: RuleKind) =>
  kind === 'restart_threshold' || kind === 'workload_unavailable_replicas';
const needsResource = (kind: RuleKind) =>
  isHealthKind(kind) || isMetricKind(kind) || isCountKind(kind);

function positiveInteger(value: string, minimum: number, maximum: number) {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= minimum && parsed <= maximum ? parsed : null;
}

function draftInput(draft: RuleDraft): AlertRuleInput | null {
  const evaluation = positiveInteger(draft.evaluationSeconds, 1, 86_400);
  const pending = positiveInteger(draft.pendingSeconds, 0, 86_400);
  if (
    draft.displayName.trim() === '' ||
    draft.sourceId.trim() === '' ||
    evaluation === null ||
    pending === null
  )
    return null;
  if (needsResource(draft.kind) && draft.resourceId.trim() === '') return null;
  const input: AlertRuleInput = {
    display_name: draft.displayName.trim(),
    enabled: draft.enabled,
    evaluation_interval_seconds: evaluation,
    kind: draft.kind,
    pending_for_seconds: pending,
    severity: draft.severity,
    source_id: draft.sourceId.trim(),
    ...(draft.environmentId.trim() === '' ? {} : { environment_id: draft.environmentId.trim() }),
    ...(needsResource(draft.kind) ? { resource_id: draft.resourceId.trim() } : {}),
  };
  if (isMetricKind(draft.kind)) {
    const threshold = Number(draft.thresholdValue);
    const window = positiveInteger(draft.windowSeconds, 1, 86_400);
    if (
      !Number.isFinite(threshold) ||
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

function recordInput(rule: AlertRuleRecord): AlertRuleInput {
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

function recordDraft(rule: AlertRuleRecord): RuleDraft {
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
    windowSeconds: String(rule.window_seconds ?? 300),
  };
}

export function ProductAlertRulesSettingsPage({ toolbar }: { readonly toolbar?: ReactNode }) {
  const api = useProductApi();
  const { t } = useLocale();
  const [rules, setRules] = useState<readonly AlertRuleRecord[]>([]);
  const [draft, setDraft] = useState<RuleDraft>(emptyDraft);
  const [editing, setEditing] = useState<AlertRuleRecord | null>(null);
  const [deleteIntent, setDeleteIntent] = useState<string | null>(null);
  const [state, setState] = useState<'loading' | 'ready' | 'saving' | 'error'>('loading');
  const [delivery, setDelivery] = useState<
    | { readonly kind: 'loading' | 'error' }
    | { readonly kind: 'ready'; readonly status: NotificationSettingsStatus }
  >({ kind: 'loading' });
  const [testPassword, setTestPassword] = useState('');
  const [testDelivery, setTestDelivery] = useState<
    | { readonly kind: 'idle' | 'sending' | 'error' }
    | { readonly kind: 'queued'; readonly notificationId: string }
  >({ kind: 'idle' });
  const input = useMemo(() => draftInput(draft), [draft]);

  useEffect(() => {
    const controller = new AbortController();
    void api
      .listAlertRules({ signal: controller.signal })
      .then((page) => {
        setRules(page.items);
        setState('ready');
      })
      .catch(() => {
        if (!controller.signal.aborted) setState('error');
      });
    return () => {
      controller.abort();
    };
  }, [api]);

  useEffect(() => {
    const controller = new AbortController();
    void api.getNotificationSettings(controller.signal).then(
      (status) => {
        setDelivery({ kind: 'ready', status });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError')) {
          setDelivery({ kind: 'error' });
        }
      },
    );
    return () => {
      controller.abort();
    };
  }, [api]);

  const update = <Key extends keyof RuleDraft>(key: Key, value: RuleDraft[Key]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const save = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (input === null || state === 'saving') return;
    setState('saving');
    try {
      const saved =
        editing === null
          ? await api.createAlertRule(input)
          : await api.replaceAlertRule(editing.id, editing.version, input);
      setRules((current) => [saved, ...current.filter((rule) => rule.id !== saved.id)]);
      setEditing(null);
      setDraft(emptyDraft());
      setState('ready');
    } catch {
      setState('error');
    }
  };

  const toggle = async (rule: AlertRuleRecord) => {
    setState('saving');
    try {
      const saved = await api.replaceAlertRule(rule.id, rule.version, {
        ...recordInput(rule),
        enabled: !rule.enabled,
      });
      setRules((current) => current.map((item) => (item.id === saved.id ? saved : item)));
      setState('ready');
    } catch {
      setState('error');
    }
  };

  const remove = async (rule: AlertRuleRecord) => {
    if (deleteIntent !== rule.id) {
      setDeleteIntent(rule.id);
      return;
    }
    setState('saving');
    try {
      await api.deleteAlertRule(rule.id, rule.version);
      setRules((current) => current.filter((item) => item.id !== rule.id));
      setDeleteIntent(null);
      setState('ready');
    } catch {
      setState('error');
    }
  };

  const sendTest = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (testPassword === '' || testDelivery.kind === 'sending') return;
    setTestDelivery({ kind: 'sending' });
    try {
      const result = await api.testNotification(testPassword);
      setTestDelivery({ kind: 'queued', notificationId: result.notification_id });
    } catch {
      setTestDelivery({ kind: 'error' });
    } finally {
      setTestPassword('');
    }
  };

  return (
    <div className="route-page alert-rule-settings min-w-0">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('alerts.rules.eyebrow')}</p>
        <h1>{t('alerts.rules.title')}</h1>
        <p>{t('alerts.rules.description')}</p>
      </header>
      {toolbar}
      <section className="notification-delivery-panel" aria-labelledby="delivery-status-title">
        <header>
          <div>
            <h2 id="delivery-status-title">{t('notifications.settings.title')}</h2>
            <p>{t('notifications.settings.description')}</p>
          </div>
          {delivery.kind === 'ready' ? (
            <span
              className={`readiness-state readiness-state--${delivery.status.enabled ? 'ready' : 'not_ready'}`}
            >
              {delivery.status.enabled
                ? t('notifications.settings.enabled')
                : t('notifications.settings.disabled')}
            </span>
          ) : null}
        </header>
        {delivery.kind === 'loading' ? (
          <p aria-live="polite">{t('notifications.settings.loading')}</p>
        ) : null}
        {delivery.kind === 'error' ? <p role="alert">{t('notifications.settings.error')}</p> : null}
        {delivery.kind === 'ready' ? (
          <>
            <dl>
              <div>
                <dt>{t('notifications.settings.adapter')}</dt>
                <dd>{t('notifications.settings.adapter.webhook')}</dd>
              </div>
              <div>
                <dt>{t('notifications.settings.delivery')}</dt>
                <dd>{t('notifications.settings.delivery.atLeastOnce')}</dd>
              </div>
              <div>
                <dt>{t('notifications.settings.destination')}</dt>
                <dd>
                  <code dir="ltr">
                    {delivery.status.enabled
                      ? delivery.status.destination_reference_id
                      : t('notifications.settings.none')}
                  </code>
                </dd>
              </div>
            </dl>
            <p className="notification-delivery-panel__note">
              {t('notifications.settings.operatorNote')}
            </p>
            {delivery.status.enabled ? (
              <form
                className="notification-delivery-panel__test"
                onSubmit={(event) => {
                  void sendTest(event);
                }}
              >
                <label>
                  <span>{t('notifications.settings.test.password')}</span>
                  <input
                    autoComplete="current-password"
                    dir="ltr"
                    maxLength={128}
                    required
                    type="password"
                    value={testPassword}
                    onChange={(event) => {
                      setTestPassword(event.target.value);
                    }}
                  />
                </label>
                <button
                  className="button button--primary"
                  disabled={testDelivery.kind === 'sending'}
                >
                  {testDelivery.kind === 'sending'
                    ? t('notifications.settings.test.sending')
                    : t('notifications.settings.test.send')}
                </button>
                <p aria-live="polite">
                  {testDelivery.kind === 'queued'
                    ? `${t('notifications.settings.test.queued')} ${testDelivery.notificationId}`
                    : testDelivery.kind === 'error'
                      ? t('notifications.settings.test.error')
                      : t('notifications.settings.test.help')}
                </p>
              </form>
            ) : null}
          </>
        ) : null}
      </section>
      <p className="alert-rule-settings__status" aria-live="polite">
        {state === 'loading'
          ? t('alerts.rules.loading')
          : state === 'saving'
            ? t('alerts.rules.saving')
            : state === 'error'
              ? t('alerts.rules.error')
              : ''}
      </p>
      <section className="alert-rule-settings__panel" aria-labelledby="rule-editor-title">
        <h2 id="rule-editor-title">
          {editing === null ? t('alerts.rules.create') : t('alerts.rules.edit')}
        </h2>
        <form
          className="alert-rule-settings__form"
          onSubmit={(event) => {
            void save(event);
          }}
        >
          <RuleField label={t('alerts.rules.name')}>
            <input
              required
              maxLength={256}
              value={draft.displayName}
              onChange={(event) => {
                update('displayName', event.target.value);
              }}
            />
          </RuleField>
          <RuleField label={t('alerts.rules.source')}>
            <input
              required
              maxLength={128}
              value={draft.sourceId}
              onChange={(event) => {
                update('sourceId', event.target.value);
              }}
            />
          </RuleField>
          <RuleField label={t('alerts.rules.environment')}>
            <input
              maxLength={128}
              value={draft.environmentId}
              onChange={(event) => {
                update('environmentId', event.target.value);
              }}
            />
          </RuleField>
          <RuleField label={t('alerts.rules.kind')}>
            <select
              value={draft.kind}
              onChange={(event) => {
                update('kind', event.target.value as RuleKind);
              }}
            >
              {kinds.map((kind) => (
                <option key={kind} value={kind}>
                  {t(`alerts.rules.kind.${kind}`)}
                </option>
              ))}
            </select>
          </RuleField>
          {needsResource(draft.kind) ? (
            <RuleField label={t('alerts.rules.resource')}>
              <input
                required
                value={draft.resourceId}
                onChange={(event) => {
                  update('resourceId', event.target.value);
                }}
              />
            </RuleField>
          ) : null}
          <RuleField label={t('alerts.rules.severity')}>
            <select
              value={draft.severity}
              onChange={(event) => {
                update('severity', event.target.value as Severity);
              }}
            >
              {severities.map((severity) => (
                <option key={severity} value={severity}>
                  {t(`alerts.rules.severity.${severity}`)}
                </option>
              ))}
            </select>
          </RuleField>
          {isMetricKind(draft.kind) || isCountKind(draft.kind) ? (
            <RuleField label={t('alerts.rules.comparison')}>
              <select
                value={draft.comparison}
                onChange={(event) => {
                  update('comparison', event.target.value as RuleDraft['comparison']);
                }}
              >
                {comparisons.map((comparison) => (
                  <option key={comparison} value={comparison}>
                    {t(`alerts.rules.comparison.${comparison}`)}
                  </option>
                ))}
              </select>
            </RuleField>
          ) : null}
          {isMetricKind(draft.kind) ? (
            <RuleField label={t('alerts.rules.thresholdValue')}>
              <input
                type="number"
                min="0"
                required
                value={draft.thresholdValue}
                onChange={(event) => {
                  update('thresholdValue', event.target.value);
                }}
              />
            </RuleField>
          ) : null}
          {isCountKind(draft.kind) ? (
            <RuleField label={t('alerts.rules.thresholdCount')}>
              <input
                type="number"
                min="1"
                step="1"
                required
                value={draft.thresholdCount}
                onChange={(event) => {
                  update('thresholdCount', event.target.value);
                }}
              />
            </RuleField>
          ) : null}
          {isMetricKind(draft.kind) || isCountKind(draft.kind) ? (
            <RuleField label={t('alerts.rules.window')}>
              <input
                type="number"
                min="1"
                max="86400"
                required
                value={draft.windowSeconds}
                onChange={(event) => {
                  update('windowSeconds', event.target.value);
                }}
              />
            </RuleField>
          ) : null}
          {draft.kind === 'event_severity_match' ? (
            <RuleField label={t('alerts.rules.minimumSeverity')}>
              <select
                value={draft.minimumSeverity}
                onChange={(event) => {
                  update('minimumSeverity', event.target.value as Severity);
                }}
              >
                {severities.map((severity) => (
                  <option key={severity} value={severity}>
                    {t(`alerts.rules.severity.${severity}`)}
                  </option>
                ))}
              </select>
            </RuleField>
          ) : null}
          <RuleField label={t('alerts.rules.evaluation')}>
            <input
              type="number"
              min="1"
              max="86400"
              required
              value={draft.evaluationSeconds}
              onChange={(event) => {
                update('evaluationSeconds', event.target.value);
              }}
            />
          </RuleField>
          <RuleField label={t('alerts.rules.pending')}>
            <input
              type="number"
              min="0"
              max="86400"
              required
              value={draft.pendingSeconds}
              onChange={(event) => {
                update('pendingSeconds', event.target.value);
              }}
            />
          </RuleField>
          <label className="alert-rule-settings__check">
            <input
              type="checkbox"
              checked={draft.enabled}
              onChange={(event) => {
                update('enabled', event.target.checked);
              }}
            />
            {t('alerts.rules.enabled')}
          </label>
          <div className="alert-rule-settings__actions">
            <button
              className="ui-button ui-button--primary ui-focus-ring"
              disabled={input === null || state === 'saving'}
              type="submit"
            >
              {editing === null ? t('alerts.rules.createAction') : t('alerts.rules.saveAction')}
            </button>
            {editing === null ? null : (
              <button
                className="ui-button ui-button--secondary ui-focus-ring"
                type="button"
                onClick={() => {
                  setEditing(null);
                  setDraft(emptyDraft());
                }}
              >
                {t('alerts.rules.cancel')}
              </button>
            )}
          </div>
        </form>
      </section>
      <section className="alert-rule-settings__panel" aria-labelledby="rule-list-title">
        <h2 id="rule-list-title">{t('alerts.rules.list')}</h2>
        {state !== 'loading' && rules.length === 0 ? <p>{t('alerts.rules.empty')}</p> : null}
        <div className="alert-rule-settings__list">
          {rules.map((rule) => (
            <article className="alert-rule-card" key={rule.id}>
              <div>
                <h3>{rule.display_name}</h3>
                <p>
                  <code dir="ltr">{rule.kind}</code> · {t(`alerts.rules.severity.${rule.severity}`)}
                </p>
                <p>{rule.enabled ? t('alerts.rules.on') : t('alerts.rules.off')}</p>
              </div>
              <div className="alert-rule-settings__actions">
                <button
                  className="ui-button ui-button--secondary ui-focus-ring"
                  disabled={state === 'saving'}
                  onClick={() => {
                    setEditing(rule);
                    setDraft(recordDraft(rule));
                  }}
                >
                  {t('alerts.rules.editAction')}
                </button>
                <button
                  className="ui-button ui-button--secondary ui-focus-ring"
                  disabled={state === 'saving'}
                  onClick={() => void toggle(rule)}
                >
                  {rule.enabled ? t('alerts.rules.disable') : t('alerts.rules.enable')}
                </button>
                <button
                  className="ui-button ui-button--danger ui-focus-ring"
                  disabled={state === 'saving'}
                  onClick={() => void remove(rule)}
                >
                  {deleteIntent === rule.id
                    ? t('alerts.rules.confirmDelete')
                    : t('alerts.rules.delete')}
                </button>
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

function RuleField({ children, label }: { readonly children: ReactNode; readonly label: string }) {
  return (
    <label className="alert-rule-settings__field">
      <span>{label}</span>
      {children}
    </label>
  );
}
