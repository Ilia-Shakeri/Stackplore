import { Drawer, DrawerContent, DrawerTrigger, IconButton } from '@stackplore/ui';
import { Menu } from '@stackplore/ui/icons';
import { useEffect, useRef } from 'react';

import { BrandLogo } from './BrandLogo';
import { NavigationLinks } from './NavigationLinks';
import { developmentNavigation, primaryNavigation, secondaryNavigation } from './navigation';
import { useLocale } from '../../i18n';

interface MobileNavProps {
  onOpenChange: (open: boolean) => void;
  open: boolean;
  pathname: string;
}

export function MobileNav({ onOpenChange, open, pathname }: MobileNavProps) {
  const navigationRequested = useRef(false);
  const { t } = useLocale();

  useEffect(() => {
    onOpenChange(false);
  }, [onOpenChange, pathname]);

  const closeForNavigation = () => {
    navigationRequested.current = true;
    onOpenChange(false);
  };

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerTrigger asChild>
        <IconButton
          className="mobile-nav__trigger"
          variant="ghost"
          aria-label={t('shell.navigation.open')}
          aria-controls="mobile-navigation"
          aria-expanded={open}
        >
          <Menu aria-hidden="true" />
        </IconButton>
      </DrawerTrigger>
      <DrawerContent
        id="mobile-navigation"
        title={t('shell.navigation.title')}
        closeLabel={t('shell.navigation.close')}
        side="start"
        size="sm"
        onCloseAutoFocus={(event) => {
          if (!navigationRequested.current) {
            return;
          }
          navigationRequested.current = false;
          event.preventDefault();
          document.getElementById('main-content')?.focus();
        }}
      >
        <div className="mobile-nav__brand">
          <BrandLogo />
        </div>
        <nav className="shell-nav" aria-label={t('shell.navigation.primary')}>
          <NavigationLinks items={primaryNavigation} onNavigate={closeForNavigation} />
        </nav>
        <nav
          className="shell-nav mobile-nav__secondary"
          aria-label={t('shell.navigation.workspace')}
        >
          <NavigationLinks items={secondaryNavigation} onNavigate={closeForNavigation} />
          {import.meta.env.DEV ? (
            <div className="app-sidebar__development">
              <NavigationLinks items={[developmentNavigation]} onNavigate={closeForNavigation} />
            </div>
          ) : null}
        </nav>
      </DrawerContent>
    </Drawer>
  );
}
