import { TechnicalText } from '@stackplore/ui';
import { Link } from '@tanstack/react-router';

import { PlannedPage } from './PlannedPage';
import { useLocale, type TextMessageKey } from '../i18n';

const settingsLinks = [
  { labelKey: 'route.settings.general', to: '/settings' },
  { labelKey: 'route.connections.title', to: '/settings/connections' },
  { labelKey: 'route.users.title', to: '/settings/users' },
  { labelKey: 'route.notifications.title', to: '/settings/notifications' },
  { labelKey: 'route.preferences.title', to: '/settings/preferences' },
] as const satisfies readonly { labelKey: TextMessageKey; to: string }[];

function SettingsNavigation() {
  const { t } = useLocale();

  return (
    <nav className="settings-nav" aria-label={t('route.settings.sectionsLabel')}>
      {settingsLinks.map((item) => (
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

export function SetupPage() {
  const { t } = useLocale();

  return (
    <PlannedPage
      eyebrow={t('route.setup.eyebrow')}
      title={t('route.setup.title')}
      description={t('route.setup.description')}
    />
  );
}

export function LoginPage() {
  const { t } = useLocale();

  return (
    <PlannedPage
      eyebrow={t('route.login.eyebrow')}
      title={t('route.login.title')}
      description={t('route.login.description')}
    />
  );
}

export function OverviewPage() {
  const { t } = useLocale();

  return (
    <PlannedPage
      eyebrow={t('route.overview.eyebrow')}
      title={t('route.overview.title')}
      description={t('route.overview.description')}
    />
  );
}

export function TopologyPage() {
  const { t } = useLocale();

  return (
    <PlannedPage title={t('route.topology.title')} description={t('route.topology.description')} />
  );
}

export function ResourcesPage() {
  const { t } = useLocale();

  return (
    <PlannedPage
      title={t('route.resources.title')}
      description={t('route.resources.description')}
    />
  );
}

interface ResourceDetailPageProps {
  id: string;
  kind: string;
  provider: string;
}

export function ResourceDetailPage({ id, kind, provider }: ResourceDetailPageProps) {
  const { t } = useLocale();

  return (
    <PlannedPage
      title={t('route.resourceDetail.title')}
      description={t('route.resourceDetail.description')}
    >
      <dl className="resource-identity" aria-label={t('route.resourceDetail.identityLabel')}>
        <div>
          <dt>{t('route.resourceDetail.provider')}</dt>
          <dd>
            <TechnicalText>{provider}</TechnicalText>
          </dd>
        </div>
        <div>
          <dt>{t('route.resourceDetail.kind')}</dt>
          <dd>
            <TechnicalText>{kind}</TechnicalText>
          </dd>
        </div>
        <div>
          <dt>{t('route.resourceDetail.identifier')}</dt>
          <dd>
            <TechnicalText>{id}</TechnicalText>
          </dd>
        </div>
      </dl>
    </PlannedPage>
  );
}

export function LogsPage() {
  const { t } = useLocale();
  return <PlannedPage title={t('route.logs.title')} description={t('route.logs.description')} />;
}

export function EventsPage() {
  const { t } = useLocale();
  return (
    <PlannedPage title={t('route.events.title')} description={t('route.events.description')} />
  );
}

export function AlertsPage() {
  const { t } = useLocale();
  return (
    <PlannedPage title={t('route.alerts.title')} description={t('route.alerts.description')} />
  );
}

export function AuditPage() {
  const { t } = useLocale();
  return <PlannedPage title={t('route.audit.title')} description={t('route.audit.description')} />;
}

export function SettingsPage() {
  const { t } = useLocale();
  return (
    <PlannedPage
      title={t('route.settings.title')}
      description={t('route.settings.description')}
      toolbar={<SettingsNavigation />}
    />
  );
}

export function ConnectionsSettingsPage() {
  const { t } = useLocale();
  return (
    <PlannedPage
      title={t('route.connections.title')}
      description={t('route.connections.description')}
      toolbar={<SettingsNavigation />}
    />
  );
}

export function UsersSettingsPage() {
  const { t } = useLocale();
  return (
    <PlannedPage
      title={t('route.users.title')}
      description={t('route.users.description')}
      toolbar={<SettingsNavigation />}
    />
  );
}

export function NotificationsSettingsPage() {
  const { t } = useLocale();
  return (
    <PlannedPage
      title={t('route.notifications.title')}
      description={t('route.notifications.description')}
      toolbar={<SettingsNavigation />}
    />
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
