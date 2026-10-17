import { Button, Dialog, DialogClose, DialogContent, Field, Input } from '@stackplore/ui';
import { useEffect, useRef, useState, type SyntheticEvent } from 'react';

import { useLocale } from '../../i18n';
import { clearPasswordFields, isValidPasswordText, readFormText } from '../auth/auth-form';
import { useProductApi } from '../product/product-context';
import { downloadConfiguration, settingsFailure, type SettingsFailure } from './settings-api';
import { sameValues, valueNames } from './settings-guards';
import { SettingsImpact } from './SettingsImpact';
import type {
  ApplicationSettings,
  ApplicationValues,
  CleanupClass,
  CleanupPreview,
  CleanupResult,
  SettingsPreview,
} from './settings-types';

export type SettingsIntent =
  | {
      readonly kind: 'update';
      readonly settings: ApplicationSettings;
      readonly requested: ApplicationValues;
    }
  | {
      readonly kind: 'cleanup';
      readonly settings: ApplicationSettings;
      readonly classes: readonly CleanupClass[];
    }
  | { readonly kind: 'export' };
export type SettingsActionResult =
  | { readonly kind: 'update'; readonly settings: ApplicationSettings }
  | { readonly kind: 'cleanup'; readonly result: CleanupResult }
  | { readonly kind: 'export' };
type PreviewState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error'; readonly failure: SettingsFailure }
  | { readonly kind: 'ready'; readonly preview: SettingsPreview | CleanupPreview | null };

export function SettingsActionDialog({
  intent,
  onClose,
  onCompleted,
  onReturnFocus,
}: {
  readonly intent: SettingsIntent;
  readonly onClose: () => void;
  readonly onCompleted: (result: SettingsActionResult) => void;
  readonly onReturnFocus: () => void;
}) {
  const api = useProductApi();
  const { t, formatDate, formatNumber } = useLocale();
  const [state, setState] = useState<PreviewState>({ kind: 'loading' });
  const [failure, setFailure] = useState<SettingsFailure | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [backup, setBackup] = useState(false);
  const [expired, setExpired] = useState(false);
  const [spent, setSpent] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const lock = useRef(false);
  const errorRegion = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    if (intent.kind === 'export') {
      setState({ kind: 'ready', preview: null });
      return () => {
        controller.abort();
      };
    }
    const load =
      intent.kind === 'update'
        ? api.previewApplicationSettings(intent.settings, intent.requested, controller.signal)
        : api.previewCleanup(intent.settings, intent.classes, controller.signal);
    void load.then(
      (preview) => {
        if (!controller.signal.aborted) setState({ kind: 'ready', preview });
      },
      (error: unknown) => {
        if (!controller.signal.aborted)
          setState({ kind: 'error', failure: settingsFailure(error) });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, intent]);

  useEffect(() => {
    if (state.kind !== 'ready' || state.preview === null) return;
    const expiresAt = Date.parse(state.preview.expires_at);
    const synchronize = () => {
      setExpired(Date.now() >= expiresAt);
    };
    synchronize();
    const timer = window.setInterval(synchronize, 1000);
    return () => {
      window.clearInterval(timer);
    };
  }, [state]);
  useEffect(() => {
    if (failure !== null) errorRegion.current?.focus();
  }, [failure]);

  const ready = state.kind === 'ready';
  const preview = ready ? state.preview : null;
  const backupRequired = preview?.impact.backup_required ?? false;
  const submit = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!ready || lock.current || spent || expired || !confirmed || (backupRequired && !backup))
      return;
    const form = event.currentTarget;
    const password = readFormText(new FormData(form), 'current_password');
    clearPasswordFields(form);
    if (!isValidPasswordText(password)) {
      setFailure('invalid');
      return;
    }
    lock.current = true;
    setSubmitting(true);
    setSpent(true);
    setFailure(null);
    try {
      if (intent.kind === 'export') {
        const data = await api.exportConfiguration(password);
        downloadConfiguration(data);
        onCompleted({ kind: 'export' });
      } else if (intent.kind === 'update' && preview !== null && 'settings' in preview) {
        const settings = await api.updateApplicationSettings(preview, password, backup);
        onCompleted({ kind: 'update', settings });
      } else if (intent.kind === 'cleanup' && preview !== null && 'policy_version' in preview) {
        const result = await api.cleanupData(preview, password, backup);
        onCompleted({ kind: 'cleanup', result });
      }
    } catch (error) {
      setFailure(settingsFailure(error));
    } finally {
      lock.current = false;
      setSubmitting(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !lock.current) onClose();
      }}
    >
      <DialogContent
        title={t(`settings.dialog.${intent.kind}`)}
        description={t('settings.dialog.description')}
        closeLabel={t('settings.dialog.close')}
        size="lg"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          onReturnFocus();
        }}
        footer={
          <>
            <DialogClose asChild>
              <Button variant="secondary" disabled={submitting}>
                {t('settings.dialog.close')}
              </Button>
            </DialogClose>
            <Button
              type="submit"
              form="application-settings-action"
              loading={submitting}
              disabled={!ready || !confirmed || (backupRequired && !backup) || expired || spent}
            >
              {t('settings.dialog.submit')}
            </Button>
          </>
        }
      >
        {state.kind === 'loading' ? <p role="status">{t('settings.loading')}</p> : null}
        {state.kind === 'error' ? <p role="alert">{t(`settings.error.${state.failure}`)}</p> : null}
        {ready ? (
          <form
            id="application-settings-action"
            onSubmit={(event) => {
              void submit(event);
            }}
            aria-busy={submitting || undefined}
            className="settings-review"
          >
            {preview !== null ? (
              <p>
                {t('settings.dialog.expires')}:{' '}
                <time dateTime={preview.expires_at}>
                  {formatDate(Date.parse(preview.expires_at), {
                    dateStyle: 'short',
                    timeStyle: 'medium',
                  })}
                </time>
              </p>
            ) : null}
            {preview !== null &&
            'settings' in preview &&
            !sameValues(preview.settings.values, preview.requested) ? (
              <div className="settings-table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">{t('settings.dialog.field')}</th>
                      <th scope="col">{t('settings.dialog.current')}</th>
                      <th scope="col">{t('settings.dialog.requested')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {valueNames
                      .filter((name) => preview.settings.values[name] !== preview.requested[name])
                      .map((name) => (
                        <tr key={name}>
                          <th scope="row">{t(`settings.${name}`)}</th>
                          <td>{formatNumber(preview.settings.values[name])}</td>
                          <td>{formatNumber(preview.requested[name])}</td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            ) : null}
            {preview !== null ? (
              <SettingsImpact impact={preview.impact} />
            ) : (
              <p>{t('settings.export.note')}</p>
            )}
            {expired ? <p role="alert">{t('settings.error.confirmation')}</p> : null}
            {failure !== null ? (
              <div role="alert" tabIndex={-1} ref={errorRegion}>
                {t(`settings.error.${failure}`)}
              </div>
            ) : null}
            {submitting ? <p role="status">{t('settings.dialog.submitting')}</p> : null}
            {backupRequired ? (
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={backup}
                  required
                  disabled={expired || spent}
                  onChange={(event) => {
                    setBackup(event.currentTarget.checked);
                  }}
                />
                {t('settings.dialog.backupConfirm')}
              </label>
            ) : null}
            <label className="settings-check">
              <input
                type="checkbox"
                checked={confirmed}
                required
                disabled={expired || spent}
                onChange={(event) => {
                  setConfirmed(event.currentTarget.checked);
                }}
              />
              {t(intent.kind === 'export' ? 'settings.export.confirm' : 'settings.dialog.confirm')}
            </label>
            <Field label={t('settings.dialog.password')} required>
              <Input
                type="password"
                name="current_password"
                autoComplete="current-password"
                dir="ltr"
                maxLength={128}
                disabled={expired || spent}
              />
            </Field>
          </form>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
