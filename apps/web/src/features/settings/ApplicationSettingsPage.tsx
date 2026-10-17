import { Button, Input, TechnicalText } from '@stackplore/ui';
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type SyntheticEvent,
} from 'react';

import { useLocale } from '../../i18n';
import { useAuth } from '../auth/auth-context';
import { useProductApi } from '../product/product-context';
import { settingsFailure, type SettingsFailure } from './settings-api';
import { isApplicationValues, sameValues, valueBounds, valueNames } from './settings-guards';
import {
  SettingsActionDialog,
  type SettingsActionResult,
  type SettingsIntent,
} from './SettingsActionDialog';
import {
  cleanupClasses,
  type ApplicationSettings,
  type ApplicationValues,
  type CleanupClass,
} from './settings-types';
import './settings.css';

type LoadState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error'; readonly failure: SettingsFailure }
  | { readonly kind: 'ready'; readonly settings: ApplicationSettings };
type Draft = Record<keyof ApplicationValues, string>;
const draftValues = (values?: ApplicationValues): Draft =>
  Object.fromEntries(
    valueNames.map((name) => [name, values === undefined ? '' : String(values[name])]),
  ) as Draft;
function parseDraft(draft: Draft): ApplicationValues | undefined {
  const candidate: Record<string, unknown> = {};
  for (const name of valueNames) {
    const text = draft[name];
    if (!/^[0-9]{1,5}$/u.test(text)) return undefined;
    candidate[name] = Number(text);
  }
  return isApplicationValues(candidate) ? candidate : undefined;
}

export function ApplicationSettingsPage({ toolbar }: { readonly toolbar?: ReactNode }) {
  const api = useProductApi();
  const { snapshot } = useAuth();
  const { t, formatDate, formatNumber } = useLocale();
  const administrator =
    snapshot.status === 'authenticated' && snapshot.user.role === 'administrator';
  const actor =
    snapshot.status === 'authenticated' ? `${snapshot.user.id}:${snapshot.user.role}` : '';
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [draft, setDraft] = useState<Draft>(() => draftValues());
  const [intent, setIntent] = useState<SettingsIntent | null>(null);
  const [result, setResult] = useState<SettingsActionResult | null>(null);
  const [scope, setScope] = useState<CleanupClass[]>(['metric_samples']);
  const [localFailure, setLocalFailure] = useState(false);
  const validDraft = parseDraft(draft);
  const controller = useRef<AbortController | null>(null);
  const generation = useRef(0);
  const actionButton = useRef<HTMLButtonElement | null>(null);
  const formError = useRef<HTMLParagraphElement>(null);
  const load = useCallback(async () => {
    controller.current?.abort();
    const current = new AbortController();
    controller.current = current;
    const turn = ++generation.current;
    setState({ kind: 'loading' });
    try {
      const settings = await api.getApplicationSettings(current.signal);
      if (current.signal.aborted || turn !== generation.current) return;
      setState({ kind: 'ready', settings });
      setDraft(draftValues(settings.values));
      setLocalFailure(false);
    } catch (error) {
      if (!current.signal.aborted && turn === generation.current)
        setState({ kind: 'error', failure: settingsFailure(error) });
    }
  }, [api]);
  useEffect(() => {
    setIntent(null);
    setResult(null);
    if (actor !== '') void load();
    return () => {
      controller.current?.abort();
      generation.current++;
    };
  }, [actor, load]);
  useEffect(() => {
    if (localFailure) formError.current?.focus();
  }, [localFailure]);

  const finish = (outcome: SettingsActionResult) => {
    setIntent(null);
    setResult(outcome);
    if (outcome.kind === 'update') {
      setState({ kind: 'ready', settings: outcome.settings });
      setDraft(draftValues(outcome.settings.values));
    } else void load();
  };
  const review = (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!administrator || state.kind !== 'ready') return;
    const requested = parseDraft(draft);
    if (requested === undefined) {
      setLocalFailure(true);
      return;
    }
    if (sameValues(requested, state.settings.values)) return;
    setResult(null);
    setIntent({ kind: 'update', settings: state.settings, requested });
  };

  if (snapshot.status !== 'authenticated') return <p role="alert">{t('settings.error.denied')}</p>;
  return (
    <div className="route-page settings-page">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('route.settings.general')}</p>
        <h1>{t('route.settings.title')}</h1>
        <p>{t('settings.description')}</p>
      </header>
      {toolbar}
      <div className="settings-actions">
        <Button
          variant="secondary"
          disabled={state.kind === 'loading' || intent !== null}
          onClick={() => {
            setResult(null);
            void load();
          }}
        >
          {t('settings.retry')}
        </Button>
      </div>
      {state.kind === 'loading' ? <p role="status">{t('settings.loading')}</p> : null}
      {state.kind === 'error' ? <p role="alert">{t(`settings.error.${state.failure}`)}</p> : null}
      {!administrator ? <p>{t('settings.viewer')}</p> : null}
      {result?.kind === 'update' ? <p role="status">{t('settings.saved')}</p> : null}
      {result?.kind === 'export' ? <p role="status">{t('settings.export.generated')}</p> : null}
      {result?.kind === 'cleanup' ? (
        <div role="status">
          <p>{t('settings.cleanup.done', { count: result.result.removed_rows })}</p>
          <p>
            {t(
              result.result.remaining_rows
                ? 'settings.cleanup.remaining'
                : 'settings.cleanup.complete',
            )}
          </p>
        </div>
      ) : null}
      {state.kind === 'ready' ? (
        <>
          <section className="settings-panel" aria-labelledby="application-policy-title">
            <h2 id="application-policy-title">{t('settings.values')}</h2>
            {administrator ? (
              <form onSubmit={review} noValidate>
                <div className="settings-fields">
                  {valueNames.map((name) => (
                    <div className="settings-field" key={name}>
                      <label htmlFor={`settings-${name}`}>{t(`settings.${name}`)}</label>
                      <Input
                        id={`settings-${name}`}
                        dir="ltr"
                        type="number"
                        min={valueBounds[name][0]}
                        max={valueBounds[name][1]}
                        step="1"
                        value={draft[name]}
                        aria-describedby={`settings-${name}-bound`}
                        onChange={(event) => {
                          const value = event.currentTarget.value;
                          if (value.length <= 7)
                            setDraft((current) => ({ ...current, [name]: value }));
                          setLocalFailure(false);
                        }}
                      />
                      <small id={`settings-${name}-bound`}>
                        {t('settings.bounds', {
                          minimum: valueBounds[name][0],
                          maximum: valueBounds[name][1],
                        })}
                      </small>
                    </div>
                  ))}
                </div>
                {localFailure ? (
                  <p role="alert" tabIndex={-1} ref={formError}>
                    {t('settings.error.invalid')}
                  </p>
                ) : null}
                <div className="settings-actions">
                  <Button
                    type="submit"
                    ref={actionButton}
                    disabled={
                      intent !== null ||
                      (validDraft !== undefined && sameValues(state.settings.values, validDraft))
                    }
                  >
                    {t('settings.previewChanges')}
                  </Button>
                </div>
              </form>
            ) : (
              <dl className="settings-facts">
                {valueNames.map((name) => (
                  <div key={name}>
                    <dt>{t(`settings.${name}`)}</dt>
                    <dd>{formatNumber(state.settings.values[name])}</dd>
                  </div>
                ))}
              </dl>
            )}
          </section>
          <section className="settings-panel" aria-labelledby="application-info-title">
            <h2 id="application-info-title">{t('settings.info')}</h2>
            <dl className="settings-facts">
              <div>
                <dt>{t('settings.version')}</dt>
                <dd>
                  <TechnicalText>{state.settings.application_version}</TechnicalText>
                </dd>
              </div>
              <div>
                <dt>{t('settings.policyVersion')}</dt>
                <dd>{formatNumber(state.settings.version)}</dd>
              </div>
              <div>
                <dt>{t('settings.updated')}</dt>
                <dd>
                  <time dateTime={state.settings.updated_at}>
                    {formatDate(Date.parse(state.settings.updated_at), {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </time>
                </dd>
              </div>
              {(['enabled', 'docker_write_enabled', 'kubernetes_write_enabled'] as const).map(
                (name, index) => (
                  <div key={name}>
                    <dt>
                      {t(
                        (
                          [
                            'settings.mutations',
                            'settings.dockerWrite',
                            'settings.kubernetesWrite',
                          ] as const
                        )[index] ?? 'settings.mutations',
                      )}
                    </dt>
                    <dd>
                      {t(
                        state.settings.mutation_mode[name]
                          ? 'settings.enabled'
                          : 'settings.disabled',
                      )}
                    </dd>
                  </div>
                ),
              )}
            </dl>
            <p>{t('settings.gatesNote')}</p>
          </section>
          <section className="settings-panel" aria-labelledby="application-audit-title">
            <h2 id="application-audit-title">{t('settings.audit')}</h2>
            <dl className="settings-facts">
              <div>
                <dt>{t('settings.audit.retention')}</dt>
                <dd>{formatNumber(state.settings.audit_policy.retention_days)}</dd>
              </div>
              <div>
                <dt>{t('settings.audit.minimum')}</dt>
                <dd>{formatNumber(state.settings.audit_policy.minimum_retention_days)}</dd>
              </div>
              <div>
                <dt>{t('settings.audit.quota')}</dt>
                <dd>{formatNumber(state.settings.audit_policy.quota_rows)}</dd>
              </div>
              <div>
                <dt>{t('settings.audit.stored')}</dt>
                <dd>{formatNumber(state.settings.audit_policy.stored_rows)}</dd>
              </div>
            </dl>
            {state.settings.audit_policy.quota_exceeded ? (
              <p role="alert">{t('settings.audit.exceeded')}</p>
            ) : null}
            <p>{t('settings.audit.note')}</p>
          </section>
          {administrator ? (
            <>
              <section className="settings-panel" aria-labelledby="application-export-title">
                <h2 id="application-export-title">{t('settings.export')}</h2>
                <p>{t('settings.export.note')}</p>
                <Button
                  variant="secondary"
                  onClick={(event) => {
                    actionButton.current = event.currentTarget;
                    setResult(null);
                    setIntent({ kind: 'export' });
                  }}
                >
                  {t('settings.export.action')}
                </Button>
              </section>
              <section className="settings-panel" aria-labelledby="application-cleanup-title">
                <h2 id="application-cleanup-title">{t('settings.cleanup')}</h2>
                <p>{t('settings.cleanup.note')}</p>
                <fieldset className="settings-cleanup-scope">
                  <legend>{t('settings.cleanup')}</legend>
                  {cleanupClasses.map((name) => (
                    <label className="settings-check" key={name}>
                      <input
                        type="checkbox"
                        checked={scope.includes(name)}
                        onChange={(event) => {
                          const checked = event.currentTarget.checked;
                          setScope((current) =>
                            checked ? [...current, name] : current.filter((item) => item !== name),
                          );
                        }}
                      />
                      {t(`settings.cleanup.${name}`)}
                    </label>
                  ))}
                </fieldset>
                <Button
                  variant="secondary"
                  disabled={scope.length === 0}
                  onClick={(event) => {
                    actionButton.current = event.currentTarget;
                    setResult(null);
                    setIntent({ kind: 'cleanup', settings: state.settings, classes: [...scope] });
                  }}
                >
                  {t('settings.cleanup.preview')}
                </Button>
              </section>
            </>
          ) : null}
        </>
      ) : null}
      {intent !== null && administrator ? (
        <SettingsActionDialog
          intent={intent}
          onClose={() => {
            setIntent(null);
          }}
          onCompleted={finish}
          onReturnFocus={() => actionButton.current?.focus()}
        />
      ) : null}
    </div>
  );
}
