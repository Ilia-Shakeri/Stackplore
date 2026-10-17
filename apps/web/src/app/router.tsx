import { createApiClient } from '@stackplore/api-client';
import {
  Navigate,
  Outlet,
  type RouterHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  useRouterState,
  useRouter,
} from '@tanstack/react-router';
import { lazy, Suspense, useCallback, useEffect } from 'react';

import { AppShell } from './shell/AppShell';
import { StandaloneShell } from './shell/StandaloneShell';
import { getPageTitle } from './shell/navigation';
import { NotFoundPage } from '../routes/NotFoundPage';
import {
  AboutPage,
  AlertsPage,
  AuditPage,
  ConnectionsSettingsPage,
  EventsPage,
  LogsPage,
  NotificationsSettingsPage,
  NotificationCenterPage,
  OverviewPage,
  PreferencesSettingsPage,
  ResourceDetailPage,
  ResourcesPage,
  SearchPage,
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
import { createProductApi, type ProductApi } from '../features/product/product-api';
import {
  ProductProvider,
  readProductQuery,
  readProductSource,
} from '../features/product/product-context';
import { createUsersApi, type UsersApi } from '../features/users/users-api';
import { UsersProvider } from '../features/users/users-context';
import { useLocale } from '../i18n';
import { productViewSearch, validViewParameter } from '../features/product/product-view-search';

interface RouterContext {
  readonly auth: AuthStore;
  readonly product: ProductApi;
  readonly users: UsersApi;
}

const defaultApiClient = createApiClient();
const defaultAuthStore = new AuthStore(createAuthApi(defaultApiClient));
const defaultUsersApi = createUsersApi(defaultApiClient, defaultAuthStore);
const defaultProductApi = createProductApi(defaultApiClient, defaultAuthStore);

const DesignSystemPage = lazy(() =>
  import('../routes/DesignSystemPage').then((module) => ({ default: module.DesignSystemPage })),
);

const rootRoute = createRootRouteWithContext<RouterContext>()({
  validateSearch: (search: Record<string, unknown>) => {
    const source =
      typeof search.source === 'string'
        ? readProductSource(new URLSearchParams({ source: search.source }).toString())
        : undefined;
    const q =
      typeof search.q === 'string'
        ? readProductQuery(new URLSearchParams({ q: search.q }).toString())
        : '';
    return {
      ...productViewSearch(search),
      ...(source === undefined ? {} : { source }),
      ...(q === '' ? {} : { q }),
    };
  },
  component: RootRoute,
  notFoundComponent: () => <Navigate to="/not-found" replace />,
});

function RootRoute() {
  const { auth, product, users } = rootRoute.useRouteContext();
  const validatedSearch = rootRoute.useSearch();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const search = useRouterState({ select: (state) => state.location.searchStr });
  const router = useRouter();
  const resourceSource = pathname.startsWith('/resources/')
    ? readProductSource(`source=${pathname.split('/')[2] ?? ''}`)
    : undefined;
  const selectedSource = resourceSource ?? readProductSource(search);
  const query = readProductQuery(search);
  const replaceSearch = useCallback(
    (name: string, value: string) => {
      const url = new URL(router.history.location.href, 'http://localhost');
      if (value === '') url.searchParams.delete(name);
      else url.searchParams.set(name, value);
      router.history.replace(`${url.pathname}${url.search}${url.hash}`);
    },
    [router],
  );
  const selectSource = useCallback(
    (sourceId: string) => {
      replaceSearch('source', sourceId);
    },
    [replaceSearch],
  );
  const selectQuery = useCallback(
    (value: string) => {
      replaceSearch('q', value);
    },
    [replaceSearch],
  );
  const selectViewParameter = useCallback(
    (name: string, value: string) => {
      if (validViewParameter(name, value)) replaceSearch(name, value);
    },
    [replaceSearch],
  );
  const { t } = useLocale();

  useEffect(() => {
    const pageTitle = getPageTitle(pathname, t);
    document.title = pageTitle === 'Stackplore' ? pageTitle : `${pageTitle} | Stackplore`;
  }, [pathname, t]);

  return (
    <AuthProvider store={auth}>
      <ProductProvider
        api={product}
        onSourceChange={selectSource}
        onQueryChange={selectQuery}
        query={query}
        viewSearch={new URLSearchParams(productViewSearch(validatedSearch)).toString()}
        onViewParameterChange={selectViewParameter}
        {...(selectedSource === undefined ? {} : { sourceId: selectedSource })}
      >
        <UsersProvider api={users}>
          <Outlet />
        </UsersProvider>
      </ProductProvider>
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

const searchRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/search',
  validateSearch: (search: Record<string, unknown>) => {
    const q =
      typeof search.q === 'string' && search.q.length <= 200 && search.q.trim() === search.q
        ? search.q
        : undefined;
    const source =
      typeof search.source === 'string' && /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(search.source)
        ? search.source
        : undefined;
    return { ...(q === undefined ? {} : { q }), ...(source === undefined ? {} : { source }) };
  },
  component: SearchRoute,
});

function SearchRoute() {
  const search = searchRoute.useSearch();
  return <SearchPage query={search.q ?? ''} />;
}

const resourceDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/resources/$source/$provider/$kind/$id',
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

const notificationsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/notifications',
  component: NotificationCenterPage,
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
  getParentRoute: () => appRoute,
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
  component: DesignSystemRoute,
});

function DesignSystemRoute() {
  const { t } = useLocale();
  if (!import.meta.env.DEV) {
    return <Navigate to="/not-found" replace />;
  }
  return (
    <Suspense fallback={<p aria-live="polite">{t('product.loading')}</p>}>
      <DesignSystemPage />
    </Suspense>
  );
}

const routeTree = rootRoute.addChildren([
  rootIndexRoute,
  standaloneRoute.addChildren([setupRoute, loginRoute]),
  appRoute.addChildren([
    overviewRoute,
    topologyRoute,
    resourcesRoute,
    searchRoute,
    resourceDetailRoute,
    logsRoute,
    eventsRoute,
    alertsRoute,
    notificationsRoute,
    settingsRoute,
    settingsPreferencesRoute,
    settingsConnectionsRoute,
    aboutRoute,
    notFoundRoute,
    designSystemRoute,
    administratorRoute.addChildren([auditRoute, settingsUsersRoute, settingsNotificationsRoute]),
  ]),
]);

export function createAppRouter(
  history?: RouterHistory,
  auth: AuthStore = defaultAuthStore,
  users: UsersApi = auth === defaultAuthStore
    ? defaultUsersApi
    : createUsersApi(createApiClient(), auth),
  product: ProductApi = auth === defaultAuthStore
    ? defaultProductApi
    : createProductApi(createApiClient(), auth),
) {
  return createRouter({
    context: { auth, product, users },
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
