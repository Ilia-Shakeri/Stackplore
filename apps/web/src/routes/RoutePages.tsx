import { Link } from '@tanstack/react-router';
import { useTheme } from '@stackplore/ui';
import { lazy, Suspense, type ReactNode } from 'react';

import type * as ProductPageExports from '../features/product/ProductPages';
import { useAuth } from '../features/auth/auth-context';
import { useLocale, type TextMessageKey } from '../i18n';

const productPage = <Name extends keyof typeof ProductPageExports>(name: Name) =>
  lazy(() =>
    import('../features/product/ProductPages').then((module) => ({ default: module[name] })),
  );

const ProductOverviewPage = productPage('ProductOverviewPage');
const ProductTopologyPage = lazy(() =>
  import('../features/product/ExplorationTopologyPage').then((module) => ({
    default: module.ProductTopologyPage,
  })),
);
const ProductResourcesPage = lazy(() =>
  import('../features/product/ExplorationResourcesPage').then((module) => ({
    default: module.ProductResourcesPage,
  })),
);
const ProductSearchPage = lazy(() =>
  import('../features/product/ExplorationResourcesPage').then((module) => ({
    default: module.ProductSearchPage,
  })),
);
const ProductResourceDetailPage = lazy(() =>
  import('../features/product/ExplorationResourceDetailPage').then((module) => ({
    default: module.ProductResourceDetailPage,
  })),
);
const ProductLogsPage = lazy(() =>
  import('../features/product/ExplorationLogsPage').then((module) => ({
    default: module.ProductLogsPage,
  })),
);
const ProductEventsPage = lazy(() =>
  import('../features/product/ExplorationEventsPage').then((module) => ({
    default: module.ProductEventsPage,
  })),
);
const ProductAlertsPage = lazy(() =>
  import('../features/product/ExplorationAlertsPage').then((module) => ({
    default: module.ProductAlertsPage,
  })),
);
const ProductNotificationsPage = lazy(() =>
  import('../features/product/ProductHistoryPages').then((module) => ({
    default: module.ProductNotificationsPage,
  })),
);
const ProductAuditPage = lazy(() =>
  import('../features/product/ProductHistoryPages').then((module) => ({
    default: module.ProductAuditPage,
  })),
);
const ProductSystemSettingsPage = lazy(() =>
  import('../features/settings/ApplicationSettingsPage').then((module) => ({
    default: module.ApplicationSettingsPage,
  })),
);
const SourceSettingsPage = lazy(() =>
  import('../features/product/SourceSettingsPage').then((module) => ({
    default: module.SourceSettingsPage,
  })),
);
const ProductAlertRulesSettingsPage = lazy(() =>
  import('../features/product/AlertRulesSettingsPage').then((module) => ({
    default: module.ProductAlertRulesSettingsPage,
  })),
);

const UserAdminPage = lazy(() =>
  import('../features/users/UserAdminPage').then((module) => ({
    default: module.UserAdminPage,
  })),
);
const LocalPreferencesPage = lazy(() =>
  import('../features/settings/PreferencesPage').then((module) => ({
    default: module.PreferencesPage,
  })),
);

const settingsLinks = [
  { authenticatedOnly: true, labelKey: 'route.settings.general', to: '/settings' },
  { administratorOnly: true, labelKey: 'route.connections.title', to: '/settings/connections' },
  { administratorOnly: true, labelKey: 'route.users.title', to: '/settings/users' },
  {
    administratorOnly: true,
    labelKey: 'alerts.rules.nav',
    to: '/settings/notifications',
  },
  { labelKey: 'route.preferences.title', to: '/settings/preferences' },
] as const satisfies readonly {
  administratorOnly?: boolean;
  authenticatedOnly?: boolean;
  labelKey: TextMessageKey;
  to: string;
}[];

function SettingsNavigation() {
  const { snapshot } = useAuth();
  const { t } = useLocale();
  const administrator =
    snapshot.status === 'authenticated' && snapshot.user.role === 'administrator';

  return (
    <nav className="settings-nav" aria-label={t('route.settings.sectionsLabel')}>
      {settingsLinks
        .filter(
          (item) =>
            (!('administratorOnly' in item) || administrator) &&
            (!('authenticatedOnly' in item) || snapshot.status === 'authenticated'),
        )
        .map((item) => (
          <Link
            key={item.to}
            className="settings-nav__link ui-focus-ring"
            to={item.to}
            activeOptions={{ exact: true }}
          >
            {t(item.labelKey)}
          </Link>
        ))}
    </nav>
  );
}

export function OverviewPage() {
  return (
    <ProductBoundary>
      <ProductOverviewPage />
    </ProductBoundary>
  );
}

export function TopologyPage() {
  return (
    <ProductBoundary>
      <ProductTopologyPage />
    </ProductBoundary>
  );
}

export function ResourcesPage() {
  return (
    <ProductBoundary>
      <ProductResourcesPage />
    </ProductBoundary>
  );
}

export function SearchPage({ query }: { readonly query: string }) {
  return (
    <ProductBoundary>
      <ProductSearchPage query={query} />
    </ProductBoundary>
  );
}

interface ResourceDetailPageProps {
  id: string;
  kind: string;
  provider: string;
  source: string;
}

export function ResourceDetailPage({ id, kind, provider, source }: ResourceDetailPageProps) {
  return (
    <ProductBoundary>
      <ProductResourceDetailPage id={id} kind={kind} provider={provider} source={source} />
    </ProductBoundary>
  );
}

export function LogsPage() {
  return (
    <ProductBoundary>
      <ProductLogsPage />
    </ProductBoundary>
  );
}

export function EventsPage() {
  return (
    <ProductBoundary>
      <ProductEventsPage />
    </ProductBoundary>
  );
}

export function AlertsPage() {
  return (
    <ProductBoundary>
      <ProductAlertsPage />
    </ProductBoundary>
  );
}

export function NotificationCenterPage() {
  return (
    <ProductBoundary>
      <ProductNotificationsPage />
    </ProductBoundary>
  );
}

function ProductBoundary({ children }: { readonly children: ReactNode }) {
  const { t } = useLocale();
  return (
    <Suspense
      fallback={
        <section className="product-state" aria-live="polite">
          <p>{t('product.loading')}</p>
        </section>
      }
    >
      {children}
    </Suspense>
  );
}

export function AuditPage() {
  return (
    <ProductBoundary>
      <ProductAuditPage />
    </ProductBoundary>
  );
}

export function SettingsPage() {
  return (
    <ProductBoundary>
      <ProductSystemSettingsPage toolbar={<SettingsNavigation />} />
    </ProductBoundary>
  );
}

export function ConnectionsSettingsPage() {
  return (
    <ProductBoundary>
      <SourceSettingsPage toolbar={<SettingsNavigation />} />
    </ProductBoundary>
  );
}

export function UsersSettingsPage() {
  const { t } = useLocale();
  return (
    <Suspense
      fallback={
        <div className="route-page users-page min-w-0">
          <header className="route-page__header">
            <p className="route-page__eyebrow">{t('users.page.eyebrow')}</p>
            <h1>{t('route.users.title')}</h1>
            <p>{t('route.users.description')}</p>
          </header>
          <SettingsNavigation />
          <section className="users-state" aria-live="polite">
            <h2>{t('users.loading.title')}</h2>
            <p>{t('users.loading.body')}</p>
          </section>
        </div>
      }
    >
      <UserAdminPage toolbar={<SettingsNavigation />} />
    </Suspense>
  );
}

export function NotificationsSettingsPage() {
  return (
    <ProductBoundary>
      <ProductAlertRulesSettingsPage toolbar={<SettingsNavigation />} />
    </ProductBoundary>
  );
}

export function PreferencesSettingsPage() {
  return (
    <ProductBoundary>
      <LocalPreferencesPage toolbar={<SettingsNavigation />} />
    </ProductBoundary>
  );
}

export function AboutPage() {
  const { t, tPlural } = useLocale();
  const { resolvedTheme } = useTheme();

  return (
    <div className="route-page">
      <header className="route-page__header about-hero">
        <div className="about-hero__copy">
          <p className="route-page__eyebrow">{t('route.about.eyebrow')}</p>
          <h1>{t('route.about.title')}</h1>
          <p>{t('route.about.description')}</p>
        </div>
        <img
          className="about-hero__image"
          src={`/brand/stackplore-intro-${resolvedTheme}.webp`}
          width="960"
          height="640"
          alt=""
          loading="lazy"
          decoding="async"
        />
      </header>

      <section className="planned-state" aria-labelledby="about-status-title">
        <div className="planned-state__mark" aria-hidden="true" />
        <div>
          <h2 id="about-status-title">{t('route.about.statusTitle')}</h2>
          <p>{t('route.about.statusBody')}</p>
          <p>{tPlural('route.about.languageCount', 2)}</p>
        </div>
      </section>
    </div>
  );
}
