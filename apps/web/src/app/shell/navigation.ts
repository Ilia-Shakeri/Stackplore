import {
  Activity,
  Bell,
  Boxes,
  CircleHelp,
  Info,
  LayoutDashboard,
  List,
  Network,
  Settings,
} from '@stackplore/ui/icons';
import type { ComponentType, SVGProps } from 'react';

type NavigationIcon = ComponentType<SVGProps<SVGSVGElement>>;

export interface NavigationItem {
  icon: NavigationIcon;
  label: string;
  to:
    | '/overview'
    | '/topology'
    | '/resources'
    | '/logs'
    | '/events'
    | '/alerts'
    | '/audit'
    | '/settings'
    | '/about'
    | '/design-system';
}

export const primaryNavigation = [
  { icon: LayoutDashboard, label: 'Overview', to: '/overview' },
  { icon: Network, label: 'Topology', to: '/topology' },
  { icon: Boxes, label: 'Resources', to: '/resources' },
  { icon: Activity, label: 'Logs', to: '/logs' },
  { icon: List, label: 'Events', to: '/events' },
  { icon: Bell, label: 'Alerts', to: '/alerts' },
  { icon: List, label: 'Audit', to: '/audit' },
] as const satisfies readonly NavigationItem[];

export const secondaryNavigation = [
  { icon: Settings, label: 'Settings', to: '/settings' },
  { icon: Info, label: 'About', to: '/about' },
] as const satisfies readonly NavigationItem[];

export const developmentNavigation = {
  icon: CircleHelp,
  label: 'Design system',
  to: '/design-system',
} as const satisfies NavigationItem;

export interface BreadcrumbItem {
  label: string;
  to?: NavigationItem['to'];
}

const fixedBreadcrumbs = new Map<string, readonly BreadcrumbItem[]>([
  ['/overview', [{ label: 'Overview' }]],
  ['/topology', [{ label: 'Topology' }]],
  ['/resources', [{ label: 'Resources' }]],
  ['/logs', [{ label: 'Logs' }]],
  ['/events', [{ label: 'Events' }]],
  ['/alerts', [{ label: 'Alerts' }]],
  ['/audit', [{ label: 'Audit' }]],
  ['/settings', [{ label: 'Settings' }]],
  ['/settings/connections', [{ label: 'Settings', to: '/settings' }, { label: 'Connections' }]],
  ['/settings/users', [{ label: 'Settings', to: '/settings' }, { label: 'Users' }]],
  ['/settings/notifications', [{ label: 'Settings', to: '/settings' }, { label: 'Notifications' }]],
  ['/settings/preferences', [{ label: 'Settings', to: '/settings' }, { label: 'Preferences' }]],
  ['/about', [{ label: 'About' }]],
  ['/design-system', [{ label: 'Design system' }]],
]);

export function getBreadcrumbs(pathname: string): readonly BreadcrumbItem[] {
  const fixed = fixedBreadcrumbs.get(pathname);
  if (fixed !== undefined) {
    return fixed;
  }
  if (pathname.startsWith('/resources/')) {
    return [{ label: 'Resources', to: '/resources' }, { label: 'Resource detail' }];
  }
  return [];
}

export function getPageTitle(pathname: string): string {
  if (pathname === '/setup') {
    return 'Setup';
  }
  if (pathname === '/login') {
    return 'Sign in';
  }
  if (pathname === '/not-found') {
    return 'Page not found';
  }
  return getBreadcrumbs(pathname).at(-1)?.label ?? 'Stackplore';
}
