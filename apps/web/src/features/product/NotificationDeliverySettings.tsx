import type { NotificationSettingsStatus } from '@stackplore/api-client';
import { useEffect, useState, type SyntheticEvent } from 'react';

import { useProductApi } from './product-context';
import { useLocale } from '../../i18n';
import { ProductState } from './exploration-ui';

export function NotificationDeliverySettings() {
  const api = useProductApi();
  const { t } = useLocale();
  const [delivery, setDelivery] = useState<
    | { readonly kind: 'loading' | 'error' }
    | { readonly kind: 'ready'; readonly status: NotificationSettingsStatus }
  >({ kind: 'loading' });
  const [testPassword, setTestPassword] = useState('');
  const [revision, setRevision] = useState(0);
  const [testAdapter, setTestAdapter] = useState<'generic_webhook' | 'matrix'>('generic_webhook');
  const [testDelivery, setTestDelivery] = useState<
    | { readonly kind: 'idle' | 'sending' | 'error' }
    | { readonly kind: 'queued'; readonly notificationId: string }
  >({ kind: 'idle' });
  useEffect(() => {
    const controller = new AbortController();
    setDelivery({ kind: 'loading' });
    void api.getNotificationSettings(controller.signal).then(
      (status) => {
        if (controller.signal.aborted) return;
        setDelivery({ kind: 'ready', status });
        if (!status.enabled && status.matrix.enabled) setTestAdapter('matrix');
      },
      (error: unknown) => {
        if (
          !controller.signal.aborted &&
          !(error instanceof DOMException && error.name === 'AbortError')
        ) {
          setDelivery({ kind: 'error' });
        }
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, revision]);

  const sendTest = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (testPassword === '' || testDelivery.kind === 'sending') return;
    setTestDelivery({ kind: 'sending' });
    try {
      const result =
        testAdapter === 'matrix'
          ? await api.testNotification(testPassword, 'matrix')
          : await api.testNotification(testPassword);
      setTestDelivery({ kind: 'queued', notificationId: result.notification_id });
    } catch {
      setTestDelivery({ kind: 'error' });
    } finally {
      setTestPassword('');
    }
  };

  return (
    <section className="notification-delivery-panel" aria-labelledby="delivery-status-title">
      <header>
        <div>
          <h2 id="delivery-status-title">{t('notifications.settings.title')}</h2>
          <p>{t('notifications.settings.description')}</p>
        </div>
        {delivery.kind === 'ready' ? (
          <span
            className={`readiness-state readiness-state--${delivery.status.enabled || delivery.status.matrix.enabled ? 'ready' : 'not_ready'}`}
          >
            {delivery.status.enabled || delivery.status.matrix.enabled
              ? t('notifications.settings.enabled')
              : t('notifications.settings.disabled')}
          </span>
        ) : null}
      </header>
      {delivery.kind === 'loading' ? (
        <p aria-live="polite">{t('notifications.settings.loading')}</p>
      ) : null}
      {delivery.kind === 'error' ? (
        <ProductState
          text={t('notifications.settings.error')}
          onRetry={() => {
            setRevision((current) => current + 1);
          }}
        />
      ) : null}
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
          <dl>
            <div>
              <dt>{t('notifications.settings.adapter.matrix')}</dt>
              <dd>
                {delivery.status.matrix.enabled
                  ? t('notifications.settings.enabled')
                  : t('notifications.settings.disabled')}
              </dd>
            </div>
            <div>
              <dt>{t('notifications.settings.destination.matrix')}</dt>
              <dd>
                <code dir="ltr">
                  {delivery.status.matrix.enabled
                    ? delivery.status.matrix.destination_reference_id
                    : t('notifications.settings.none')}
                </code>
              </dd>
            </div>
          </dl>
          <p className="notification-delivery-panel__note">
            {t('notifications.settings.operatorNote')}
          </p>
          {delivery.status.enabled || delivery.status.matrix.enabled ? (
            <form
              className="notification-delivery-panel__test"
              onSubmit={(event) => {
                void sendTest(event);
              }}
            >
              <label>
                <span>{t('notifications.settings.test.adapter')}</span>
                <select
                  value={testAdapter}
                  disabled={testDelivery.kind === 'sending'}
                  onChange={(event) => {
                    setTestAdapter(event.target.value === 'matrix' ? 'matrix' : 'generic_webhook');
                    setTestDelivery({ kind: 'idle' });
                  }}
                >
                  <option value="generic_webhook" disabled={!delivery.status.enabled}>
                    {t('notifications.settings.adapter.webhook')}
                  </option>
                  <option value="matrix" disabled={!delivery.status.matrix.enabled}>
                    {t('notifications.settings.adapter.matrix')}
                  </option>
                </select>
              </label>
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
              <button className="button button--primary" disabled={testDelivery.kind === 'sending'}>
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
  );
}
