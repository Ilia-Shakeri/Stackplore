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

import type { UserRole } from '../../features/auth/auth-api';
import type { TextMessageKey } from '../../i18n';

type NavigationIcon = ComponentType<SVGProps<SVGSVGElement>>;
type MessageTranslator = (key: TextMessageKey) => string;

export interface NavigationItem {
  icon: NavigationIcon;
  labelKey: TextMessageKey;
  to:
    | '/overview'
    | '/topology'
    | '/resources'
    | '/search'
    | '/logs'
    | '/events'
    | '/alerts'
    | '/notifications'
    | '/audit'
    | '/settings'
    | '/settings/preferences'
    | '/about'
    | '/design-system';
}

const primaryNavigation = [
  { icon: LayoutDashboard, labelKey: 'route.overview.title', to: '/overview' },
  { icon: Network, labelKey: 'route.topology.title', to: '/topology' },
  { icon: Boxes, labelKey: 'route.resources.title', to: '/resources' },
  { icon: Activity, labelKey: 'route.logs.title', to: '/logs' },
  { icon: List, labelKey: 'route.events.title', to: '/events' },
  { icon: Bell, labelKey: 'route.alerts.title', to: '/alerts' },
  { icon: List, labelKey: 'route.audit.title', to: '/audit' },
] as const satisfies readonly NavigationItem[];

export const secondaryNavigation = [
  { icon: Settings, labelKey: 'route.settings.title', to: '/settings' },
  { icon: Info, labelKey: 'route.about.title', to: '/about' },
] as const satisfies readonly NavigationItem[];

const viewerSecondaryNavigation = [
  { icon: Settings, labelKey: 'route.settings.title', to: '/settings/preferences' },
  { icon: Info, labelKey: 'route.about.title', to: '/about' },
] as const satisfies readonly NavigationItem[];

export function getPrimaryNavigation(role: UserRole): readonly NavigationItem[] {
  return role === 'administrator'
    ? primaryNavigation
    : primaryNavigation.filter((item) => item.to !== '/audit');
}

export function getSecondaryNavigation(role: UserRole): readonly NavigationItem[] {
  return role === 'administrator' ? secondaryNavigation : viewerSecondaryNavigation;
}

export const developmentNavigation = {
  icon: CircleHelp,
  labelKey: 'route.designSystem.title',
  to: '/design-system',
} as const satisfies NavigationItem;

export interface BreadcrumbItem {
  label: string;
  to?: NavigationItem['to'];
}

interface BreadcrumbDefinition {
  labelKey: TextMessageKey;
  to?: NavigationItem['to'];
}

const fixedBreadcrumbs = new Map<string, readonly BreadcrumbDefinition[]>([
  ['/overview', [{ labelKey: 'route.overview.title' }]],
  ['/topology', [{ labelKey: 'route.topology.title' }]],
  ['/resources', [{ labelKey: 'route.resources.title' }]],
  ['/search', [{ labelKey: 'route.search.title' }]],
  ['/logs', [{ labelKey: 'route.logs.title' }]],
  ['/events', [{ labelKey: 'route.events.title' }]],
  ['/alerts', [{ labelKey: 'route.alerts.title' }]],
  ['/notifications', [{ labelKey: 'route.notificationCenter.title' }]],
  ['/audit', [{ labelKey: 'route.audit.title' }]],
  ['/settings', [{ labelKey: 'route.settings.title' }]],
  [
    '/settings/connections',
    [
      { labelKey: 'route.settings.title', to: '/settings' },
      { labelKey: 'route.connections.title' },
    ],
  ],
  [
    '/settings/users',
    [{ labelKey: 'route.settings.title', to: '/settings' }, { labelKey: 'route.users.title' }],
  ],
  [
    '/settings/notifications',
    [
      { labelKey: 'route.settings.title', to: '/settings' },
      { labelKey: 'route.notifications.title' },
    ],
  ],
  [
    '/settings/preferences',
    [
      { labelKey: 'route.settings.title', to: '/settings' },
      { labelKey: 'route.preferences.title' },
    ],
  ],
  ['/about', [{ labelKey: 'route.about.title' }]],
  ['/design-system', [{ labelKey: 'route.designSystem.title' }]],
]);

function translateBreadcrumbs(
  breadcrumbs: readonly BreadcrumbDefinition[],
  t: MessageTranslator,
): readonly BreadcrumbItem[] {
  return breadcrumbs.map((item) => ({
    label: t(item.labelKey),
    ...(item.to === undefined ? {} : { to: item.to }),
  }));
}

export function getBreadcrumbs(pathname: string, t: MessageTranslator): readonly BreadcrumbItem[] {
  const fixed = fixedBreadcrumbs.get(pathname);
  if (fixed !== undefined) {
    return translateBreadcrumbs(fixed, t);
  }
  if (pathname.startsWith('/resources/')) {
    return translateBreadcrumbs(
      [
        { labelKey: 'route.resources.title', to: '/resources' },
        { labelKey: 'route.resourceDetail.title' },
      ],
      t,
    );
  }
  return [];
}

export function getPageTitle(pathname: string, t: MessageTranslator): string {
  if (pathname === '/setup') {
    return t('route.setup.title');
  }
  if (pathname === '/login') {
    return t('route.login.title');
  }
  if (pathname === '/not-found') {
    return t('route.notFound.title');
  }
  return getBreadcrumbs(pathname, t).at(-1)?.label ?? 'Stackplore';
}
