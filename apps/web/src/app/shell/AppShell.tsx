import { Outlet, useRouterState } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';

import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';
import { useLocale } from '../../i18n';

export function AppShell() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const mainRef = useRef<HTMLElement>(null);
  const previousPath = useRef(pathname);
  const { t } = useLocale();

  useEffect(() => {
    if (previousPath.current !== pathname) {
      previousPath.current = pathname;
      mainRef.current?.focus();
    }
  }, [pathname]);

  return (
    <div
      className="app-shell min-h-dvh bg-canvas text-ink min-w-0"
      data-sidebar-collapsed={sidebarCollapsed || undefined}
    >
      <a className="skip-link" href="#main-content">
        {t('shell.skipToMain')}
      </a>
      <Sidebar collapsed={sidebarCollapsed} onCollapsedChange={setSidebarCollapsed} />
      <div className="app-shell__body min-w-0">
        <TopBar />
        <main ref={mainRef} id="main-content" className="app-shell__main" tabIndex={-1}>
          <div className="app-shell__page-frame min-w-0">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
