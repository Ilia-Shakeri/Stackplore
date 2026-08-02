import { createApiClient } from '@stackplore/api-client';
import {
  Navigate,
  Outlet,
  type RouterHistory,
  createRootRouteWithContext,
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
  LogsPage,
  NotificationsSettingsPage,
  OverviewPage,
  PreferencesSettingsPage,
  ResourceDetailPage,
  ResourcesPage,
  SettingsPage,
  TopologyPage,
  UsersSettingsPage,
} from '../routes/RoutePages';
import { AuthBootPage } from '../features/auth/AuthBootPage';
import { LoginPage } from '../features/auth/LoginPage';
import { SetupPage } from '../features/auth/SetupPage';
import { createAuthApi } from '../features/auth/auth-api';
import { AuthProvider, useAuth } from '../features/auth/auth-context';
import { AuthStore } from '../features/auth/auth-store';
import { createUsersApi, type UsersApi } from '../features/users/users-api';
import { UsersProvider } from '../features/users/users-context';
import { useLocale } from '../i18n';

interface RouterContext {
  readonly auth: AuthStore;
  readonly users: UsersApi;
}

const defaultApiClient = createApiClient();
const defaultAuthStore = new AuthStore(createAuthApi(defaultApiClient));
const defaultUsersApi = createUsersApi(defaultApiClient, defaultAuthStore);

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootRoute,
  notFoundComponent: () => <Navigate to="/not-found" replace />,
});

function RootRoute() {
  const { auth, users } = rootRoute.useRouteContext();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useLocale();

  useEffect(() => {
    const pageTitle = getPageTitle(pathname, t);
    document.title = pageTitle === 'Stackplore' ? pageTitle : `${pageTitle} | Stackplore`;
  }, [pathname, t]);

  return (
    <AuthProvider store={auth}>
      <UsersProvider api={users}>
        <Outlet />
      </UsersProvider>
    </AuthProvider>
  );
}

const rootIndexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: RootIndexRoute,
});

function RootIndexRoute() {
  const { snapshot } = useAuth();
  if (snapshot.status === 'loading' || snapshot.status === 'unavailable') {
    return <AuthBootPage />;
  }
  if (snapshot.status === 'setup_required') {
    return <Navigate to="/setup" replace />;
  }
  return <Navigate to={snapshot.status === 'authenticated' ? '/overview' : '/login'} replace />;
}

const standaloneRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'standalone',
  component: StandaloneShell,
});

const setupRoute = createRoute({
  getParentRoute: () => standaloneRoute,
  path: '/setup',
  component: SetupRoute,
});

function SetupRoute() {
  const { snapshot } = useAuth();
  if (snapshot.status === 'loading' || snapshot.status === 'unavailable') {
    return <AuthBootPage />;
  }
  if (snapshot.status === 'authenticated') {
    return <Navigate to="/overview" replace />;
  }
  if (snapshot.status === 'unauthenticated') {
    return <Navigate to="/login" replace />;
  }
  return <SetupPage />;
}

export function sanitizeNextPath(value: unknown): string | undefined {
  if (
    typeof value !== 'string' ||
    value.length === 0 ||
    value.length > 2048 ||
    !value.startsWith('/') ||
    value.startsWith('//') ||
    value.includes('\\') ||
    /[\p{Cc}\p{Zl}\p{Zp}]/u.test(value)
  ) {
    return undefined;
  }
  const path = value.split(/[?#]/u, 1)[0];
  if (path === '/login' || path === '/setup') {
    return undefined;
  }
  return value;
}

const loginRoute = createRoute({
  getParentRoute: () => standaloneRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>) => {
    const next = sanitizeNextPath(search.next);
    return next === undefined ? {} : { next };
  },
  component: LoginRoute,
});

function LoginRoute() {
  const search = loginRoute.useSearch();
  const { snapshot } = useAuth();
  if (snapshot.status === 'loading' || snapshot.status === 'unavailable') {
    return <AuthBootPage />;
  }
  if (snapshot.status === 'setup_required') {
    return <Navigate to="/setup" replace />;
  }
  if (snapshot.status === 'authenticated') {
    return <Navigate to="/overview" replace />;
  }
  return <LoginPage {...(search.next === undefined ? {} : { next: search.next })} />;
}

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: AuthenticatedAppRoute,
});

function AuthenticatedAppRoute() {
  const href = useRouterState({ select: (state) => state.location.href });
  const { snapshot } = useAuth();
  if (snapshot.status === 'loading' || snapshot.status === 'unavailable') {
    return <AuthBootPage />;
  }
  if (snapshot.status === 'setup_required') {
    return <Navigate to="/setup" replace />;
  }
  if (snapshot.status === 'unauthenticated') {
    const next = sanitizeNextPath(href);
    return <Navigate to="/login" search={next === undefined ? {} : { next }} replace />;
  }
  return <AppShell />;
}

const administratorRoute = createRoute({
  getParentRoute: () => appRoute,
  id: 'administrator',
  component: AdministratorRoute,
});

function AdministratorRoute() {
  const { snapshot } = useAuth();
  if (snapshot.status !== 'authenticated' || snapshot.user.role !== 'administrator') {
    return <Navigate to="/overview" replace />;
  }
  return <Outlet />;
}

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
  getParentRoute: () => administratorRoute,
  path: '/audit',
  component: AuditPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings',
  component: SettingsEntryRoute,
});

function SettingsEntryRoute() {
  const { snapshot } = useAuth();
  if (snapshot.status === 'authenticated' && snapshot.user.role === 'viewer') {
    return <Navigate to="/settings/preferences" replace />;
  }
  return <SettingsPage />;
}

const settingsConnectionsRoute = createRoute({
  getParentRoute: () => administratorRoute,
  path: '/settings/connections',
  component: ConnectionsSettingsPage,
});

const settingsUsersRoute = createRoute({
  getParentRoute: () => administratorRoute,
  path: '/settings/users',
  component: UsersSettingsPage,
});

const settingsNotificationsRoute = createRoute({
  getParentRoute: () => administratorRoute,
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
    settingsRoute,
    settingsPreferencesRoute,
    aboutRoute,
    notFoundRoute,
    designSystemRoute,
    administratorRoute.addChildren([
      auditRoute,
      settingsConnectionsRoute,
      settingsUsersRoute,
      settingsNotificationsRoute,
    ]),
  ]),
]);

export function createAppRouter(
  history?: RouterHistory,
  auth: AuthStore = defaultAuthStore,
  users: UsersApi = auth === defaultAuthStore
    ? defaultUsersApi
    : createUsersApi(createApiClient(), auth),
) {
  return createRouter({
    context: { auth, users },
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
