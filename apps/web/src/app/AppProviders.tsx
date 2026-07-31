import { ThemeProvider, ToastProvider, TooltipProvider } from '@stackplore/ui';
import type { ReactNode } from 'react';

import { useLocale } from '../i18n';

interface AppProvidersProps {
  children: ReactNode;
}

export function AppProviders({ children }: AppProvidersProps) {
  const { direction, t } = useLocale();

  return (
    <ThemeProvider defaultTheme="system" storageKey="stackplore.theme">
      <TooltipProvider>
        <ToastProvider
          viewportLabel={t('app.toast.viewport')}
          closeLabel={t('app.toast.dismiss')}
          label={t('app.toast.label')}
          swipeDirection={direction === 'rtl' ? 'left' : 'right'}
        >
          {children}
        </ToastProvider>
      </TooltipProvider>
    </ThemeProvider>
  );
}
