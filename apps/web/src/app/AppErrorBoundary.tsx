import { Button, ErrorBoundary } from '@stackplore/ui';
import type { ReactNode } from 'react';

interface AppErrorBoundaryProps {
  children: ReactNode;
}

export function AppErrorBoundary({ children }: AppErrorBoundaryProps) {
  return (
    <ErrorBoundary
      fallback={({ resetErrorBoundary }) => (
        <main className="fatal-error">
          <section className="fatal-error__panel" role="alert" aria-labelledby="fatal-error-title">
            <p className="route-page__eyebrow">Application error</p>
            <h1 id="fatal-error-title">The workspace could not continue</h1>
            <p>Reload the current page. No provider action was sent.</p>
            <Button
              onClick={() => {
                resetErrorBoundary();
                window.location.reload();
              }}
            >
              Reload page
            </Button>
          </section>
        </main>
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
