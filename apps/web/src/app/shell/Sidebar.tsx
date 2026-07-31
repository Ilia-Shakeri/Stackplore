import { IconButton, Tooltip } from '@stackplore/ui';
import { PanelLeftClose, PanelLeftOpen } from '@stackplore/ui/icons';

import { BrandLogo } from './BrandLogo';
import { NavigationLinks } from './NavigationLinks';
import { developmentNavigation, primaryNavigation, secondaryNavigation } from './navigation';
import { useLocale } from '../../i18n';

interface SidebarProps {
  collapsed: boolean;
  onCollapsedChange: (collapsed: boolean) => void;
}

export function Sidebar({ collapsed, onCollapsedChange }: SidebarProps) {
  const { direction, t } = useLocale();
  const collapseLabel = collapsed ? t('shell.sidebar.expand') : t('shell.sidebar.collapse');
  const CollapseIcon = collapsed ? PanelLeftOpen : PanelLeftClose;

  return (
    <aside className="app-sidebar" data-collapsed={collapsed || undefined}>
      <div className="app-sidebar__header">
        <BrandLogo compact={collapsed} />
        <Tooltip content={collapseLabel} side={direction === 'rtl' ? 'left' : 'right'}>
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
            <CollapseIcon data-ui-mirror-inline aria-hidden="true" />
          </IconButton>
        </Tooltip>
      </div>

      <nav id="primary-navigation" className="shell-nav" aria-label={t('shell.navigation.primary')}>
        <NavigationLinks collapsed={collapsed} items={primaryNavigation} />
      </nav>

      <nav
        className="shell-nav app-sidebar__secondary"
        aria-label={t('shell.navigation.workspace')}
      >
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
