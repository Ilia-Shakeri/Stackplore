import type { SourceSettingsRecord, SourceUpdatePreview } from '@stackplore/api-client';
import {
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  Field,
  Input,
  TechnicalText,
} from '@stackplore/ui';
import { useEffect, useRef, useState, type SyntheticEvent } from 'react';

import { useProductApi } from './product-context';
import { classifySourceSettingsFailure, type SourceSettingsFailure } from './source-settings-api';
import { sourceSettingsFailureKey } from './source-settings-messages';
import { clearPasswordFields, isValidPasswordText, readFormText } from '../auth/auth-form';
import { useLocale } from '../../i18n';

export type SourceUpdateIntent = {
  readonly source: SourceSettingsRecord;
  readonly enabled: boolean;
};
type PreviewState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error'; readonly failure: SourceSettingsFailure }
  | { readonly kind: 'ready'; readonly preview: SourceUpdatePreview };

export function SourceUpdateDialog({
  intent,
  onClose,
  onUpdated,
  onReload,
  onReturnFocus,
}: {
  readonly intent: SourceUpdateIntent;
  readonly onClose: () => void;
  readonly onUpdated: (source: SourceSettingsRecord) => void;
  readonly onReload: () => void;
  readonly onReturnFocus: () => void;
}) {
  const api = useProductApi();
  const { formatDate, formatNumber, t } = useLocale();
  const [state, setState] = useState<PreviewState>({ kind: 'loading' });
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [spent, setSpent] = useState(false);
  const [expired, setExpired] = useState(false);
  const [failure, setFailure] = useState<SourceSettingsFailure | null>(null);
  const submitLock = useRef(false);
  const error = useRef<HTMLDivElement>(null);
  const formId = 'source-update-confirmation';

  useEffect(() => {
    const controller = new AbortController();
    void api.previewSourceUpdate(intent.source, intent.enabled, controller.signal).then(
      (preview) => {
        if (!controller.signal.aborted) setState({ kind: 'ready', preview });
      },
      (reason: unknown) => {
        if (!controller.signal.aborted)
          setState({ kind: 'error', failure: classifySourceSettingsFailure(reason) });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, intent]);

  useEffect(() => {
    if (state.kind !== 'ready') return;
    const syncExpiry = () => {
      setExpired(Date.now() >= Date.parse(state.preview.expires_at));
    };
    syncExpiry();
    const timer = globalThis.setInterval(syncExpiry, 1000);
    return () => {
      globalThis.clearInterval(timer);
    };
  }, [state]);

  useEffect(() => {
    if (failure !== null) error.current?.focus();
  }, [failure]);

  const submit = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (
      state.kind !== 'ready' ||
      !confirmed ||
      spent ||
      submitLock.current ||
      Date.now() >= Date.parse(state.preview.expires_at)
    )
      return;
    const form = event.currentTarget;
    const password = readFormText(new FormData(form), 'current_password');
    if (!isValidPasswordText(password)) {
      setFailure('invalid');
      clearPasswordFields(form);
      return;
    }
    submitLock.current = true;
    setSpent(true);
    setSubmitting(true);
    setFailure(null);
    try {
      const updated = await api.updateSource(state.preview, password);
      onUpdated(updated);
    } catch (reason) {
      setFailure(classifySourceSettingsFailure(reason));
    } finally {
      clearPasswordFields(form);
      submitLock.current = false;
      setSubmitting(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !submitLock.current) onClose();
      }}
    >
      <DialogContent
        title={t(intent.enabled ? 'connections.preview.enable' : 'connections.preview.disable')}
        description={t('connections.preview.description')}
        closeLabel={t('connections.preview.close')}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          onReturnFocus();
        }}
        footer={
          <>
            <DialogClose asChild>
              <Button disabled={submitting} variant="outline">
                {t('connections.preview.cancel')}
              </Button>
            </DialogClose>
            {state.kind === 'ready' && !spent && !expired ? (
              <Button form={formId} type="submit" loading={submitting} disabled={!confirmed}>
                {t('connections.preview.apply')}
              </Button>
            ) : null}
            {!submitting && (spent || expired || state.kind === 'error') ? (
              <Button variant="outline" onClick={onReload}>
                {t('connections.preview.reviewAgain')}
              </Button>
            ) : null}
          </>
        }
      >
        {state.kind === 'loading' ? <p role="status">{t('connections.preview.loading')}</p> : null}
        {state.kind === 'error' ? (
          <p role="alert">{t(sourceSettingsFailureKey(state.failure))}</p>
        ) : null}
        {state.kind === 'ready' ? (
          <form
            id={formId}
            className="users-dialog__form"
            onSubmit={(event) => {
              void submit(event);
            }}
            aria-busy={submitting || undefined}
          >
            <dl className="resource-identity">
              <div>
                <dt>{t('connections.preview.target')}</dt>
                <dd>
                  {state.preview.source.name}{' '}
                  <TechnicalText>{state.preview.source.id}</TechnicalText>
                </dd>
              </div>
              <div>
                <dt>{t('connections.preview.version')}</dt>
                <dd>{formatNumber(state.preview.source.version)}</dd>
              </div>
              <div>
                <dt>{t('connections.preview.expires')}</dt>
                <dd>
                  {formatDate(Date.parse(state.preview.expires_at), {
                    dateStyle: 'short',
                    timeStyle: 'medium',
                  })}
                </dd>
              </div>
            </dl>
            <div className="users-dialog__impact">
              <strong>{t('connections.preview.impact')}</strong>
              <p>
                {t(
                  state.preview.requested_enabled
                    ? 'connections.preview.startImpact'
                    : 'connections.preview.stopImpact',
                )}
              </p>
              <p>{t('connections.preview.providerUnchanged')}</p>
            </div>
            {expired ? <p role="alert">{t('connections.error.confirmation')}</p> : null}
            {failure === null ? null : (
              <div className="auth-form__error" role="alert" tabIndex={-1} ref={error}>
                {t(sourceSettingsFailureKey(failure))}
              </div>
            )}
            {submitting ? <p role="status">{t('connections.preview.saving')}</p> : null}
            <label className="alert-rule-settings__check">
              <input
                type="checkbox"
                checked={confirmed}
                disabled={spent || expired}
                required
                onChange={(event) => {
                  setConfirmed(event.target.checked);
                }}
              />
              {t(
                state.preview.requested_enabled
                  ? 'connections.preview.confirmEnable'
                  : 'connections.preview.confirmDisable',
                { name: state.preview.source.name },
              )}
            </label>
            <Field label={t('connections.preview.password')} required>
              <Input
                autoComplete="current-password"
                name="current_password"
                type="password"
                dir="ltr"
                maxLength={128}
                disabled={spent || expired}
              />
            </Field>
          </form>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
