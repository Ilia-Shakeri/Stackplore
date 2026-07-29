import { Drawer, DrawerContent, DrawerTrigger, IconButton } from '@stackplore/ui';
import { Menu } from '@stackplore/ui/icons';
import { useEffect, useRef } from 'react';

import { BrandLogo } from './BrandLogo';
import { NavigationLinks } from './NavigationLinks';
import { developmentNavigation, primaryNavigation, secondaryNavigation } from './navigation';

interface MobileNavProps {
  onOpenChange: (open: boolean) => void;
  open: boolean;
  pathname: string;
}

export function MobileNav({ onOpenChange, open, pathname }: MobileNavProps) {
  const navigationRequested = useRef(false);

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
          aria-label="Open navigation"
          aria-controls="mobile-navigation"
          aria-expanded={open}
        >
          <Menu aria-hidden="true" />
        </IconButton>
      </DrawerTrigger>
      <DrawerContent
        id="mobile-navigation"
        title="Navigation"
        closeLabel="Close navigation"
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
        <nav className="shell-nav" aria-label="Primary navigation">
          <NavigationLinks items={primaryNavigation} onNavigate={closeForNavigation} />
        </nav>
        <nav className="shell-nav mobile-nav__secondary" aria-label="Workspace navigation">
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
