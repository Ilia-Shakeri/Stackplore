import { Outlet } from '@tanstack/react-router';

import { BrandLogo } from './BrandLogo';
import { ThemeControl } from './ThemeControl';

export function StandaloneShell() {
  return (
    <div className="standalone-shell min-h-dvh bg-canvas text-ink">
      <a className="skip-link" href="#main-content">
        Skip to main content
      </a>
      <header className="standalone-shell__header">
        <BrandLogo />
        <ThemeControl />
      </header>
      <main id="main-content" className="standalone-shell__main" tabIndex={-1}>
        <Outlet />
      </main>
    </div>
  );
}
