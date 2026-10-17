import type {
  ApplicationSettingsResponse,
  ApplicationSettingsValues,
  ApplicationSettingsPreview,
  HistoryCleanupImpact,
  HistoryCleanupClass,
  CleanupPreview as WireCleanupPreview,
  CleanupResult as WireCleanupResult,
  ConfigurationExport as WireConfigurationExport,
} from '@stackplore/api-client';

export const cleanupClasses = [
  'metric_samples',
  'health_history',
  'infrastructure_events',
  'terminal_notification_deliveries',
] as const satisfies readonly HistoryCleanupClass[];
export type CleanupClass = (typeof cleanupClasses)[number];

export type ApplicationValues = ApplicationSettingsValues;
export type ApplicationSettings = ApplicationSettingsResponse;
export type CleanupImpact = HistoryCleanupImpact;
export type SettingsPreview = ApplicationSettingsPreview;
export type CleanupPreview = WireCleanupPreview;
export type CleanupResult = WireCleanupResult;
export type ConfigurationExport = WireConfigurationExport;

export interface ApplicationSettingsApi {
  getApplicationSettings(signal?: AbortSignal): Promise<ApplicationSettings>;
  previewApplicationSettings(
    settings: ApplicationSettings,
    requested: ApplicationValues,
    signal?: AbortSignal,
  ): Promise<SettingsPreview>;
  updateApplicationSettings(
    preview: SettingsPreview,
    currentPassword: string,
    backupAcknowledged: boolean,
  ): Promise<ApplicationSettings>;
  exportConfiguration(currentPassword: string): Promise<ConfigurationExport>;
  previewCleanup(
    settings: ApplicationSettings,
    classes: readonly CleanupClass[],
    signal?: AbortSignal,
  ): Promise<CleanupPreview>;
  cleanupData(
    preview: CleanupPreview,
    currentPassword: string,
    backupAcknowledged: boolean,
  ): Promise<CleanupResult>;
}
