import { Button, ErrorBoundary } from '@stackplore/ui';
import type { ReactNode } from 'react';

import { useLocale } from '../i18n';

interface AppErrorBoundaryProps {
  children: ReactNode;
}

export function AppErrorBoundary({ children }: AppErrorBoundaryProps) {
  const { t } = useLocale();

  return (
    <ErrorBoundary
      fallback={({ resetErrorBoundary }) => (
        <main className="fatal-error">
          <section className="fatal-error__panel" role="alert" aria-labelledby="fatal-error-title">
            <p className="route-page__eyebrow">{t('app.error.eyebrow')}</p>
            <h1 id="fatal-error-title">{t('app.error.title')}</h1>
            <p>{t('app.error.body')}</p>
            <Button
              onClick={() => {
                resetErrorBoundary();
                window.location.reload();
              }}
            >
              {t('app.error.reload')}
            </Button>
          </section>
        </main>
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
