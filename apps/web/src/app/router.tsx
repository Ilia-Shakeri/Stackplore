import {
  Navigate,
  Outlet,
  type RouterHistory,
  createRootRoute,
  createRoute,
  createRouter,
  useRouterState,
} from '@tanstack/react-router';
import { useEffect } from 'react';

import { AppShell } from './shell/AppShell';
import { StandaloneShell } from './shell/StandaloneShell';
import { getPageTitle } from './shell/navigation';
import { DesignSystemPage } from '../routes/DesignSystemPage';
import { NotFoundPage } from '../routes/NotFoundPage';
import {
  AboutPage,
  AlertsPage,
  AuditPage,
  ConnectionsSettingsPage,
  EventsPage,
  LoginPage,
  LogsPage,
  NotificationsSettingsPage,
  OverviewPage,
  PreferencesSettingsPage,
  ResourceDetailPage,
  ResourcesPage,
  SettingsPage,
  SetupPage,
  TopologyPage,
  UsersSettingsPage,
} from '../routes/RoutePages';
import { useLocale } from '../i18n';

const rootRoute = createRootRoute({
  component: RootRoute,
  notFoundComponent: () => <Navigate to="/not-found" replace />,
});

function RootRoute() {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useLocale();

  useEffect(() => {
    const pageTitle = getPageTitle(pathname, t);
    document.title = pageTitle === 'Stackplore' ? pageTitle : `${pageTitle} | Stackplore`;
  }, [pathname, t]);

  return <Outlet />;
}

const rootIndexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => <Navigate to="/overview" replace />,
});

const standaloneRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'standalone',
  component: StandaloneShell,
});

const setupRoute = createRoute({
  getParentRoute: () => standaloneRoute,
  path: '/setup',
  component: SetupPage,
});

const loginRoute = createRoute({
  getParentRoute: () => standaloneRoute,
  path: '/login',
  component: LoginPage,
});

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: AppShell,
});

const overviewRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/overview',
  component: OverviewPage,
});

const topologyRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/topology',
  component: TopologyPage,
});

const resourcesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/resources',
  component: ResourcesPage,
});

const resourceDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/resources/$provider/$kind/$id',
  component: ResourceDetailRoute,
});

function ResourceDetailRoute() {
  const params = resourceDetailRoute.useParams();
  return <ResourceDetailPage {...params} />;
}

const logsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/logs',
  component: LogsPage,
});

const eventsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/events',
  component: EventsPage,
});

const alertsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/alerts',
  component: AlertsPage,
});

const auditRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/audit',
  component: AuditPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings',
  component: SettingsPage,
});

const settingsConnectionsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings/connections',
  component: ConnectionsSettingsPage,
});

const settingsUsersRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings/users',
  component: UsersSettingsPage,
});

const settingsNotificationsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings/notifications',
  component: NotificationsSettingsPage,
});

const settingsPreferencesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings/preferences',
  component: PreferencesSettingsPage,
});

const aboutRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/about',
  component: AboutPage,
});

const notFoundRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/not-found',
  component: NotFoundPage,
});

const designSystemRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/design-system',
  component: () =>
    import.meta.env.DEV ? <DesignSystemPage /> : <Navigate to="/not-found" replace />,
});

const routeTree = rootRoute.addChildren([
  rootIndexRoute,
  standaloneRoute.addChildren([setupRoute, loginRoute]),
  appRoute.addChildren([
    overviewRoute,
    topologyRoute,
    resourcesRoute,
    resourceDetailRoute,
    logsRoute,
    eventsRoute,
    alertsRoute,
    auditRoute,
    settingsRoute,
    settingsConnectionsRoute,
    settingsUsersRoute,
    settingsNotificationsRoute,
    settingsPreferencesRoute,
    aboutRoute,
    notFoundRoute,
    designSystemRoute,
  ]),
]);

export function createAppRouter(history?: RouterHistory) {
  return createRouter({
    routeTree,
    defaultPreload: 'intent',
    scrollRestoration: history === undefined,
    ...(history === undefined ? {} : { history }),
  });
}

export type AppRouter = ReturnType<typeof createAppRouter>;

export const router = createAppRouter();

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
