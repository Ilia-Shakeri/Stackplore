import type {
  ApplicationSettings,
  CleanupPreview,
  CleanupResult,
  ConfigurationExport,
  SettingsPreview,
} from './settings-types';

export const settingsFixture: ApplicationSettings = {
  version: 1,
  updated_at: '2026-08-16T08:00:00Z',
  application_version: '1.0.1',
  values: {
    metric_retention_hours: 168,
    health_retention_hours: 720,
    event_retention_hours: 720,
    delivery_retention_hours: 720,
    log_line_limit: 200,
    default_alert_window_seconds: 300,
    default_alert_pending_seconds: 0,
  },
  mutation_mode: { enabled: false, docker_write_enabled: false, kubernetes_write_enabled: false },
  audit_policy: {
    retention_days: 365,
    minimum_retention_days: 90,
    quota_rows: 1000,
    stored_rows: 12,
    quota_exceeded: false,
    archive_verified_through: null,
  },
};
export const settingsPreviewFixture: SettingsPreview = {
  settings: settingsFixture,
  requested: { ...settingsFixture.values, metric_retention_hours: 1 },
  confirmation_token: 'A'.repeat(43),
  expires_at: '2099-08-16T08:00:00Z',
  impact: {
    classes: [
      {
        name: 'metric_samples',
        cutoff: '2026-08-16T07:00:00Z',
        eligible_rows: 10001,
        eligible_rows_capped: true,
      },
    ],
    maximum_rows: 1000,
    backup_required: true,
    expiry_policy_changed: true,
  },
};
export const cleanupPreviewFixture: CleanupPreview = {
  policy_version: 1,
  confirmation_token: 'A'.repeat(43),
  expires_at: '2099-08-16T08:00:00Z',
  impact: {
    classes: [
      {
        name: 'metric_samples',
        cutoff: '2026-08-09T08:00:00Z',
        eligible_rows: 25,
        eligible_rows_capped: false,
      },
    ],
    maximum_rows: 1000,
    backup_required: true,
    expiry_policy_changed: false,
  },
};
export const cleanupResultFixture: CleanupResult = {
  removed_rows: 25,
  remaining_rows: false,
  scope: ['metric_samples'],
  cutoffs: [
    { name: 'metric_samples', cutoff: cleanupPreviewFixture.impact.classes[0]?.cutoff ?? '' },
  ],
  policy_version: 1,
};
export const exportFixture: ConfigurationExport = {
  application_version: '1.0.1',
  settings: settingsFixture.values,
  sources: [
    {
      provider: 'docker',
      name: 'Fixture Engine',
      enabled: true,
      profile: 'docker_restricted_proxy',
      credential_reference: '[masked]',
      host: '[masked]',
    },
  ],
  notification_integrations: [
    { kind: 'webhook', enabled: false, target: '[masked]', credential_reference: '[masked]' },
  ],
};
