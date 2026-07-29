import { Link } from '@tanstack/react-router';

import { PlannedPage } from './PlannedPage';

const settingsLinks = [
  { label: 'General', to: '/settings' },
  { label: 'Connections', to: '/settings/connections' },
  { label: 'Users', to: '/settings/users' },
  { label: 'Notifications', to: '/settings/notifications' },
  { label: 'Preferences', to: '/settings/preferences' },
] as const;

function SettingsNavigation() {
  return (
    <nav className="settings-nav" aria-label="Settings sections">
      {settingsLinks.map((item) => (
        <Link
          key={item.to}
          className="settings-nav__link ui-focus-ring"
          to={item.to}
          activeOptions={{ exact: true }}
        >
          {item.label}
        </Link>
      ))}
    </nav>
  );
}

export function SetupPage() {
  return (
    <PlannedPage
      eyebrow="First-run route"
      title="Setup"
      description="Secure first-run setup is not available in this phase."
    />
  );
}

export function LoginPage() {
  return (
    <PlannedPage
      eyebrow="Session route"
      title="Sign in"
      description="Session-backed sign-in is not available in this phase."
    />
  );
}

export function OverviewPage() {
  return (
    <PlannedPage
      eyebrow="Workspace"
      title="Overview"
      description="The responsive shell is ready for provider-backed summaries."
    />
  );
}

export function TopologyPage() {
  return (
    <PlannedPage
      title="Topology"
      description="Topology discovery and relationship data are not connected yet."
    />
  );
}

export function ResourcesPage() {
  return (
    <PlannedPage
      title="Resources"
      description="Resource inventory is waiting for provider contracts and discovery."
    />
  );
}

interface ResourceDetailPageProps {
  id: string;
  kind: string;
  provider: string;
}

export function ResourceDetailPage({ id, kind, provider }: ResourceDetailPageProps) {
  return (
    <PlannedPage
      title="Resource detail"
      description="Resource detail is waiting for provider contracts and discovery."
    >
      <dl className="resource-identity" aria-label="Requested resource identity">
        <div>
          <dt>Provider</dt>
          <dd>{provider}</dd>
        </div>
        <div>
          <dt>Kind</dt>
          <dd>{kind}</dd>
        </div>
        <div>
          <dt>Identifier</dt>
          <dd>{id}</dd>
        </div>
      </dl>
    </PlannedPage>
  );
}

export function LogsPage() {
  return <PlannedPage title="Logs" description="Bounded log streams are not connected yet." />;
}

export function EventsPage() {
  return <PlannedPage title="Events" description="Provider event streams are not connected yet." />;
}

export function AlertsPage() {
  return (
    <PlannedPage title="Alerts" description="Alert rules and delivery are not available yet." />
  );
}

export function AuditPage() {
  return (
    <PlannedPage title="Audit" description="Guarded operations and audit history arrive later." />
  );
}

export function SettingsPage() {
  return (
    <PlannedPage
      title="Settings"
      description="Account and workspace settings are not available yet."
      toolbar={<SettingsNavigation />}
    />
  );
}

export function ConnectionsSettingsPage() {
  return (
    <PlannedPage
      title="Connections"
      description="Provider connection setup is not available yet."
      toolbar={<SettingsNavigation />}
    />
  );
}

export function UsersSettingsPage() {
  return (
    <PlannedPage
      title="Users"
      description="User administration is not available yet."
      toolbar={<SettingsNavigation />}
    />
  );
}

export function NotificationsSettingsPage() {
  return (
    <PlannedPage
      title="Notifications"
      description="Notification delivery settings are not available yet."
      toolbar={<SettingsNavigation />}
    />
  );
}

export function PreferencesSettingsPage() {
  return (
    <PlannedPage
      title="Preferences"
      description="Personal display preferences beyond local theme are not available yet."
      toolbar={<SettingsNavigation />}
    />
  );
}

export function AboutPage() {
  return (
    <div className="route-page">
      <header className="route-page__header">
        <p className="route-page__eyebrow">Stackplore</p>
        <h1>About</h1>
        <p>A local-first workspace for understanding infrastructure relationships.</p>
      </header>

      <section className="planned-state" aria-labelledby="about-status-title">
        <div className="planned-state__mark" aria-hidden="true" />
        <div>
          <h2 id="about-status-title">Foundation build</h2>
          <p>
            The repository currently provides its application shell and design foundations. No
            provider is connected.
          </p>
        </div>
      </section>
    </div>
  );
}
