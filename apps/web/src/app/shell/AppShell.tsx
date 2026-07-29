import { Outlet, useRouterState } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';

import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

export function AppShell() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const mainRef = useRef<HTMLElement>(null);
  const previousPath = useRef(pathname);

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
        Skip to main content
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
