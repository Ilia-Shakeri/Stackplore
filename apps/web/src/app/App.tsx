import { RouterProvider } from '@tanstack/react-router';

import { AppErrorBoundary } from './AppErrorBoundary';
import { AppProviders } from './AppProviders';
import { router as defaultRouter, type AppRouter } from './router';
import '../styles/app.css';

interface AppProps {
  router?: AppRouter;
}

export function App({ router = defaultRouter }: AppProps) {
  return (
    <AppErrorBoundary>
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    </AppErrorBoundary>
  );
}
