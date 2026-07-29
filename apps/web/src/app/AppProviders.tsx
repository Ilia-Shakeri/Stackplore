import { ThemeProvider, ToastProvider, TooltipProvider } from '@stackplore/ui';
import type { ReactNode } from 'react';

interface AppProvidersProps {
  children: ReactNode;
}

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <ThemeProvider defaultTheme="system" storageKey="stackplore.theme">
      <TooltipProvider>
        <ToastProvider viewportLabel="Notifications" closeLabel="Dismiss notification">
          {children}
        </ToastProvider>
      </TooltipProvider>
    </ThemeProvider>
  );
}
