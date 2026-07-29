import { IconButton, Tooltip } from '@stackplore/ui';
import { PanelLeftClose, PanelLeftOpen } from '@stackplore/ui/icons';

import { BrandLogo } from './BrandLogo';
import { NavigationLinks } from './NavigationLinks';
import { developmentNavigation, primaryNavigation, secondaryNavigation } from './navigation';

interface SidebarProps {
  collapsed: boolean;
  onCollapsedChange: (collapsed: boolean) => void;
}

export function Sidebar({ collapsed, onCollapsedChange }: SidebarProps) {
  const collapseLabel = collapsed ? 'Expand sidebar' : 'Collapse sidebar';
  const CollapseIcon = collapsed ? PanelLeftOpen : PanelLeftClose;

  return (
    <aside className="app-sidebar" data-collapsed={collapsed || undefined}>
      <div className="app-sidebar__header">
        <BrandLogo compact={collapsed} />
        <Tooltip content={collapseLabel} side="right">
          <IconButton
            className="app-sidebar__collapse"
            variant="ghost"
            aria-label={collapseLabel}
            aria-controls="primary-navigation"
            aria-expanded={!collapsed}
            onClick={() => {
              onCollapsedChange(!collapsed);
            }}
          >
            <CollapseIcon aria-hidden="true" />
          </IconButton>
        </Tooltip>
      </div>

      <nav id="primary-navigation" className="shell-nav" aria-label="Primary navigation">
        <NavigationLinks collapsed={collapsed} items={primaryNavigation} />
      </nav>

      <nav className="shell-nav app-sidebar__secondary" aria-label="Workspace navigation">
        <NavigationLinks collapsed={collapsed} items={secondaryNavigation} />
        {import.meta.env.DEV ? (
          <div className="app-sidebar__development">
            <NavigationLinks collapsed={collapsed} items={[developmentNavigation]} />
          </div>
        ) : null}
      </nav>
    </aside>
  );
}
