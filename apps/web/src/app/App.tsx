import { RouterProvider } from '@tanstack/react-router';

import { AppErrorBoundary } from './AppErrorBoundary';
import { AppProviders } from './AppProviders';
import { router as defaultRouter, type AppRouter } from './router';
import { LocaleProvider } from '../i18n';
import { PublicShellStatus } from '../pwa/PublicShellStatus';
import '../styles/app.css';

interface AppProps {
  router?: AppRouter;
}

export function App({ router = defaultRouter }: AppProps) {
  return (
    <LocaleProvider>
      <AppErrorBoundary>
        <PublicShellStatus />
        <AppProviders>
          <RouterProvider router={router} />
        </AppProviders>
      </AppErrorBoundary>
    </LocaleProvider>
  );
}
