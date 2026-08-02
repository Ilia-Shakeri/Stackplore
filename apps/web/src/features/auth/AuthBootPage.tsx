import { Button, Card, CardContent, CardDescription, CardHeader } from '@stackplore/ui';
import { useRouter } from '@tanstack/react-router';
import { useState } from 'react';

import { useAuth } from './auth-context';
import { useLocale } from '../../i18n';

export function AuthBootPage() {
  const { retry, snapshot } = useAuth();
  const { t } = useLocale();
  const router = useRouter();
  const [retrying, setRetrying] = useState(false);
  const unavailable = snapshot.status === 'unavailable';

  const handleRetry = async () => {
    setRetrying(true);
    await retry();
    await router.invalidate();
    setRetrying(false);
  };

  return (
    <div className="auth-boot" aria-live="polite">
      <Card className="auth-card" tone="raised">
        <CardHeader>
          <p className="route-page__eyebrow">Stackplore</p>
          <h1 className="ui-card__title">
            {t(unavailable ? 'auth.boot.unavailableTitle' : 'auth.boot.loadingTitle')}
          </h1>
          <CardDescription>
            {t(unavailable ? 'auth.boot.unavailableBody' : 'auth.boot.loadingBody')}
          </CardDescription>
        </CardHeader>
        {unavailable ? (
          <CardContent>
            <Button loading={retrying} onClick={() => void handleRetry()}>
              {t('auth.boot.retry')}
            </Button>
          </CardContent>
        ) : (
          <div className="auth-boot__progress" role="status">
            <span className="ui-sr-only">{t('auth.boot.loadingStatus')}</span>
          </div>
        )}
      </Card>
    </div>
  );
}
