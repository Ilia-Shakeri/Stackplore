import { Link } from '@tanstack/react-router';
import { lazy, Suspense, type ReactNode } from 'react';

import { PlannedPage } from './PlannedPage';
import type * as ProductPageExports from '../features/product/ProductPages';
import { useAuth } from '../features/auth/auth-context';
import { useLocale, type TextMessageKey } from '../i18n';

const productPage = <Name extends keyof typeof ProductPageExports>(name: Name) =>
  lazy(() =>
    import('../features/product/ProductPages').then((module) => ({ default: module[name] })),
  );

const ProductOverviewPage = productPage('ProductOverviewPage');
const ProductTopologyPage = productPage('ProductTopologyPage');
const ProductResourcesPage = productPage('ProductResourcesPage');
const ProductSearchPage = productPage('ProductSearchPage');
const ProductResourceDetailPage = productPage('ProductResourceDetailPage');
const ProductLogsPage = productPage('ProductLogsPage');
const ProductEventsPage = productPage('ProductEventsPage');
const ProductAlertsPage = productPage('ProductAlertsPage');
const ProductNotificationsPage = productPage('ProductNotificationsPage');
const ProductAuditPage = productPage('ProductAuditPage');
const ProductSystemSettingsPage = productPage('ProductSystemSettingsPage');
const ProductConnectionsSettingsPage = productPage('ProductConnectionsSettingsPage');
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

const settingsLinks = [
  { administratorOnly: true, labelKey: 'route.settings.general', to: '/settings' },
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
        .filter((item) => !('administratorOnly' in item) || administrator)
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
      <ProductConnectionsSettingsPage toolbar={<SettingsNavigation />} />
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
  const { t } = useLocale();
  return (
    <PlannedPage
      title={t('route.preferences.title')}
      description={t('route.preferences.description')}
      toolbar={<SettingsNavigation />}
    />
  );
}

export function AboutPage() {
  const { t, tPlural } = useLocale();

  return (
    <div className="route-page">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('route.about.eyebrow')}</p>
        <h1>{t('route.about.title')}</h1>
        <p>{t('route.about.description')}</p>
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
