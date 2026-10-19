import { sliceCodePoints, type AlertRuleRecord } from '@stackplore/api-client';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type SyntheticEvent,
} from 'react';

import { NotificationDeliverySettings } from './NotificationDeliverySettings';
import {
  kinds,
  severities,
  comparisons,
  emptyDraft,
  isMetricKind,
  isCountKind,
  needsResource,
  draftInput,
  recordInput,
  recordDraft,
  type RuleDraft,
  type RuleKind,
  type Severity,
  type RuleDefaults,
} from './alert-rule-draft';

import { useProductApi, useProductView } from './product-context';
import { useLocale } from '../../i18n';

export function ProductAlertRulesSettingsPage({ toolbar }: { readonly toolbar?: ReactNode }) {
  const api = useProductApi();
  const { search, selectParameter } = useProductView();
  const urlCursor = readRuleCursor(search);
  const { t } = useLocale();
  const [cursor, setCursor] = useState<string | undefined>();
  const [previousCursors, setPreviousCursors] = useState<Array<string | undefined>>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const loadController = useRef<AbortController | null>(null);
  const loadGeneration = useRef(0);
  const [rules, setRules] = useState<readonly AlertRuleRecord[]>([]);
  const [draft, setDraft] = useState<RuleDraft>(() => emptyDraft());
  const [defaults, setDefaults] = useState<
    | { readonly kind: 'loading' | 'error' }
    | { readonly kind: 'ready'; readonly value: RuleDefaults }
  >({ kind: 'loading' });
  const defaultsController = useRef<AbortController | null>(null);
  const defaultsValue = useRef<RuleDefaults | undefined>(undefined);
  const editedFields = useRef(new Set<keyof RuleDraft>());
  const draftMode = useRef<'new' | 'editing'>('new');
  const [editing, setEditing] = useState<AlertRuleRecord | null>(null);
  const [deleteIntent, setDeleteIntent] = useState<string | null>(null);
  const [state, setState] = useState<'loading' | 'ready' | 'saving' | 'error'>('loading');
  const input = useMemo(() => draftInput(draft), [draft]);

  const loadDefaults = useCallback(async () => {
    defaultsController.current?.abort();
    const controller = new AbortController();
    defaultsController.current = controller;
    setDefaults({ kind: 'loading' });
    try {
      const settings = await api.getApplicationSettings(controller.signal);
      if (controller.signal.aborted) return;
      const value = {
        windowSeconds: settings.values.default_alert_window_seconds,
        pendingSeconds: settings.values.default_alert_pending_seconds,
      };
      defaultsValue.current = value;
      setDefaults({ kind: 'ready', value });
      if (draftMode.current === 'new') {
        setDraft((current) =>
          draftMode.current === 'editing'
            ? current
            : {
                ...current,
                ...(editedFields.current.has('windowSeconds')
                  ? {}
                  : { windowSeconds: String(value.windowSeconds) }),
                ...(editedFields.current.has('pendingSeconds')
                  ? {}
                  : { pendingSeconds: String(value.pendingSeconds) }),
              },
        );
      }
    } catch {
      if (!controller.signal.aborted) setDefaults({ kind: 'error' });
    }
  }, [api]);
  useEffect(() => {
    void loadDefaults();
    return () => {
      defaultsController.current?.abort();
    };
  }, [loadDefaults]);

  const resetDraft = () => {
    draftMode.current = 'new';
    editedFields.current.clear();
    setEditing(null);
    setDraft(emptyDraft(defaultsValue.current));
  };

  const loadPage = useCallback(
    async (pageCursor?: string) => {
      loadController.current?.abort();
      const controller = new AbortController();
      loadController.current = controller;
      const generation = ++loadGeneration.current;
      setState('loading');
      try {
        const page = await api.listAlertRules({
          limit: 100,
          signal: controller.signal,
          ...(pageCursor === undefined ? {} : { cursor: pageCursor }),
        });
        if (controller.signal.aborted || generation !== loadGeneration.current) return false;
        if (page.page.has_more && page.page.next_cursor === pageCursor)
          throw new Error('Rule page did not advance.');
        setRules(page.items);
        setNextCursor(page.page.has_more ? page.page.next_cursor : undefined);
        setCursor(pageCursor);
        setState('ready');
        return true;
      } catch {
        if (!controller.signal.aborted && generation === loadGeneration.current) setState('error');
        return false;
      }
    },
    [api],
  );

  useEffect(() => {
    void loadPage(urlCursor);
    return () => {
      loadController.current?.abort();
      loadGeneration.current++;
    };
  }, [loadPage, urlCursor]);

  const nextPage = () => {
    if (nextCursor === undefined || state !== 'ready') return;
    setState('loading');
    setPreviousCursors((current) => [...current.slice(-63), cursor]);
    selectParameter('rule_cursor', nextCursor);
    setDeleteIntent(null);
  };

  const previousPage = () => {
    if (state === 'loading' || state === 'saving' || urlCursor === undefined) return;
    setState('loading');
    selectParameter('rule_cursor', previousCursors.at(-1) ?? '');
    setPreviousCursors((current) => current.slice(0, -1));
    setDeleteIntent(null);
  };

  const update = <Key extends keyof RuleDraft>(key: Key, value: RuleDraft[Key]) => {
    editedFields.current.add(key);
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const save = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (
      input === null ||
      state === 'saving' ||
      state === 'loading' ||
      (editing === null && defaults.kind !== 'ready')
    )
      return;
    const generation = loadGeneration.current;
    setState('saving');
    try {
      const saved =
        editing === null
          ? await api.createAlertRule(input)
          : await api.replaceAlertRule(editing.id, editing.version, input);
      if (generation !== loadGeneration.current) return;
      if (editing === null) {
        setPreviousCursors([]);
        selectParameter('rule_cursor', '');
        if (urlCursor === undefined) await loadPage();
      } else {
        setRules((current) => current.map((rule) => (rule.id === saved.id ? saved : rule)));
      }
      resetDraft();
      if (editing !== null) setState('ready');
    } catch {
      if (generation === loadGeneration.current) setState('error');
    }
  };

  const toggle = async (rule: AlertRuleRecord) => {
    const generation = loadGeneration.current;
    setState('saving');
    try {
      const saved = await api.replaceAlertRule(rule.id, rule.version, {
        ...recordInput(rule),
        enabled: !rule.enabled,
      });
      if (generation !== loadGeneration.current) return;
      setRules((current) => current.map((item) => (item.id === saved.id ? saved : item)));
      setState('ready');
    } catch {
      if (generation === loadGeneration.current) setState('error');
    }
  };

  const remove = async (rule: AlertRuleRecord) => {
    if (deleteIntent !== rule.id) {
      setDeleteIntent(rule.id);
      return;
    }
    const generation = loadGeneration.current;
    setState('saving');
    try {
      await api.deleteAlertRule(rule.id, rule.version);
      if (generation !== loadGeneration.current) return;
      setRules((current) => current.filter((item) => item.id !== rule.id));
      if (editing?.id === rule.id) {
        resetDraft();
      }
      setDeleteIntent(null);
      setState('ready');
    } catch {
      if (generation === loadGeneration.current) setState('error');
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
      <NotificationDeliverySettings />
      <p className="alert-rule-settings__status" aria-live="polite">
        {state === 'loading'
          ? t('alerts.rules.loading')
          : state === 'saving'
            ? t('alerts.rules.saving')
            : state === 'error'
              ? t('alerts.rules.error')
              : ''}
      </p>
      {state === 'error' ? (
        <button
          className="ui-button ui-button--secondary ui-focus-ring"
          onClick={() => {
            void loadPage(urlCursor);
          }}
        >
          {t('alerts.rules.reload')}
        </button>
      ) : null}
      <section className="alert-rule-settings__panel" aria-labelledby="rule-editor-title">
        {defaults.kind === 'loading' ? (
          <p role="status">{t('alerts.rules.defaults.loading')}</p>
        ) : null}
        {defaults.kind === 'error' ? (
          <div role="alert">
            <p>{t('alerts.rules.defaults.error')}</p>
            <button
              type="button"
              className="ui-button ui-focus-ring"
              onClick={() => {
                void loadDefaults();
              }}
            >
              {t('alerts.rules.defaults.retry')}
            </button>
          </div>
        ) : null}
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
              maxLength={512}
              value={draft.displayName}
              onChange={(event) => {
                update('displayName', sliceCodePoints(event.target.value, 256));
              }}
            />
          </RuleField>
          <RuleField label={t('alerts.rules.source')}>
            <input
              dir="ltr"
              required
              maxLength={128}
              value={draft.sourceId}
              readOnly={editing !== null}
              onChange={(event) => {
                update('sourceId', event.target.value);
              }}
            />
          </RuleField>
          {draft.kind !== 'provider_connection_lost' ? (
            <RuleField label={t('alerts.rules.environment')}>
              <input
                dir="ltr"
                maxLength={128}
                value={draft.environmentId}
                onChange={(event) => {
                  update('environmentId', event.target.value);
                }}
              />
            </RuleField>
          ) : null}
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
          {needsResource(draft.kind) || draft.kind === 'event_severity_match' ? (
            <RuleField label={t('alerts.rules.resource')}>
              <input
                required={needsResource(draft.kind)}
                dir="ltr"
                maxLength={46}
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
                step="any"
                dir="ltr"
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
              disabled={
                input === null ||
                state === 'saving' ||
                state === 'loading' ||
                (editing === null && defaults.kind !== 'ready')
              }
              type="submit"
            >
              {editing === null ? t('alerts.rules.createAction') : t('alerts.rules.saveAction')}
            </button>
            {editing === null ? null : (
              <button
                className="ui-button ui-button--secondary ui-focus-ring"
                type="button"
                disabled={state === 'saving' || state === 'loading'}
                onClick={() => {
                  resetDraft();
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
        {state === 'ready' && rules.length === 0 ? <p>{t('alerts.rules.empty')}</p> : null}
        <div
          className="alert-rule-settings__list"
          aria-busy={state === 'loading' || state === 'saving'}
        >
          {rules.map((rule) => (
            <article className="alert-rule-card" key={rule.id}>
              <div>
                <h3>{rule.display_name}</h3>
                <p>
                  <span>{t(`alerts.rules.kind.${rule.kind}`)}</span> ·{' '}
                  {t(`alerts.rules.severity.${rule.severity}`)}
                </p>
                <p>{rule.enabled ? t('alerts.rules.on') : t('alerts.rules.off')}</p>
              </div>
              <div className="alert-rule-settings__actions">
                <button
                  className="ui-button ui-button--secondary ui-focus-ring"
                  disabled={state === 'saving' || state === 'loading'}
                  onClick={() => {
                    draftMode.current = 'editing';
                    setEditing(rule);
                    setDraft(recordDraft(rule));
                  }}
                >
                  {t('alerts.rules.editAction')}
                </button>
                <button
                  className="ui-button ui-button--secondary ui-focus-ring"
                  disabled={state === 'saving' || state === 'loading'}
                  onClick={() => void toggle(rule)}
                >
                  {rule.enabled ? t('alerts.rules.disable') : t('alerts.rules.enable')}
                </button>
                <button
                  className="ui-button ui-button--danger ui-focus-ring"
                  disabled={state === 'saving' || state === 'loading'}
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
        <nav aria-label={t('alerts.rules.pagination')} className="alert-rule-settings__actions">
          <button
            className="ui-button ui-button--secondary ui-focus-ring"
            disabled={urlCursor === undefined || state === 'loading' || state === 'saving'}
            onClick={() => {
              previousPage();
            }}
          >
            {previousCursors.length === 0
              ? t('alerts.rules.firstPage')
              : t('alerts.rules.previousPage')}
          </button>
          <button
            className="ui-button ui-button--secondary ui-focus-ring"
            disabled={nextCursor === undefined || state !== 'ready'}
            onClick={() => {
              nextPage();
            }}
          >
            {t('alerts.rules.nextPage')}
          </button>
        </nav>
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

function readRuleCursor(search: string) {
  const value = new URLSearchParams(search).get('rule_cursor');
  return value !== null && /^[A-Za-z0-9_-]{1,4096}$/u.test(value) ? value : undefined;
}
