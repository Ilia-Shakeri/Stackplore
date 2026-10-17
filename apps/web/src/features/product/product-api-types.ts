import type {
  AlertRuleInput,
  AlertRulePage,
  AlertRuleRecord,
  ProductAlert,
  ProductAlertPage,
  ProductAuditPage,
  ProductEnvironmentPage,
  ProductEventPage,
  ProductLogPage,
  ProductMetricHistoryResult,
  ProductMetricResult,
  ProductNotificationPage,
  NotificationSettingsStatus,
  NotificationTestResponse,
  ProductRelationshipPage,
  ProductResource,
  ProductResourceHealth,
  ProductResourcePage,
  SourceSummary,
  SystemReadiness,
} from '@stackplore/api-client';
import type { SessionProtection } from '../auth/auth-api';
import type { SourceSettingsApi } from './source-settings-api';

export interface ProductApi extends SourceSettingsApi {
  acknowledgeAlert(alertId: string): Promise<ProductAlert>;
  createAlertRule(input: AlertRuleInput): Promise<AlertRuleRecord>;
  deleteAlertRule(ruleId: string, version: number): Promise<void>;
  getResource(
    resourceId: string,
    signal?: AbortSignal,
    sourceId?: string,
  ): Promise<ProductResource>;
  getResourceHealth(
    resourceId: string,
    signal?: AbortSignal,
    sourceId?: string,
  ): Promise<ProductResourceHealth>;
  getNotificationSettings(signal?: AbortSignal): Promise<NotificationSettingsStatus>;
  getSystemReadiness(signal?: AbortSignal): Promise<SystemReadiness>;
  listAlerts(options?: ProductPageOptions): Promise<ProductAlertPage>;
  listAlertRules(options?: GlobalPageOptions): Promise<AlertRulePage>;
  listAudit(options?: GlobalPageOptions): Promise<ProductAuditPage>;
  listEnvironments(options?: ProductPageOptions): Promise<ProductEnvironmentPage>;
  listEvents(options?: ObservationReadOptions): Promise<ProductEventPage>;
  listLogs(options?: LogReadOptions): Promise<ProductLogPage>;
  listMetricHistory(options?: MetricHistoryOptions): Promise<ProductMetricHistoryResult>;
  listMetrics(options?: ResourceSelectionOptions): Promise<ProductMetricResult>;
  listNotifications(options?: GlobalPageOptions): Promise<ProductNotificationPage>;
  listResources(options?: ResourceReadOptions): Promise<ProductResourcePage>;
  listSources(signal?: AbortSignal): Promise<readonly SourceSummary[]>;
  listTopology(options?: ResourcePageReadOptions): Promise<ProductRelationshipPage>;
  replaceAlertRule(
    ruleId: string,
    version: number,
    input: AlertRuleInput,
  ): Promise<AlertRuleRecord>;
  testNotification(
    currentPassword: string,
    adapter?: 'generic_webhook' | 'matrix',
  ): Promise<NotificationTestResponse>;
}

export interface SessionActionRunner {
  runRecentAction?<Result>(
    purpose: 'notification.test' | 'source.update',
    currentPassword: string,
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result>;
  runSessionAction<Result>(
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result>;
}

export interface ProductPageOptions {
  readonly cursor?: string;
  readonly limit?: number;
  readonly signal?: AbortSignal;
  readonly sourceId?: string;
}

export interface ResourceReadOptions extends ProductPageOptions {
  readonly search?: string;
  readonly provider?: ProductResource['provider'];
  readonly environmentId?: string;
  readonly kind?: ProductResource['kind'];
  readonly health?: NonNullable<ProductResource['health']>;
  readonly availability?: ProductResourcePage['availability'];
  readonly scopeKind?: ProductResource['scope_kind'];
  readonly scopeName?: string;
  readonly sort?: 'name' | 'kind' | 'observed_at';
  readonly direction?: 'asc' | 'desc';
}

export type GlobalPageOptions = Pick<ProductPageOptions, 'cursor' | 'limit' | 'signal'>;

export interface ResourceSelectionOptions {
  readonly resourceId?: string;
  readonly signal?: AbortSignal;
  readonly sourceId?: string;
}

export interface ResourcePageReadOptions extends ProductPageOptions, ResourceSelectionOptions {}

export interface ObservationReadOptions extends ResourcePageReadOptions {
  readonly from?: string;
  readonly to?: string;
}

export interface LogReadOptions extends ObservationReadOptions {
  readonly container?: string;
  readonly previous?: boolean;
}

export interface MetricHistoryOptions extends ResourceSelectionOptions {
  readonly from?: string;
  readonly to?: string;
  readonly limit?: number;
  readonly minutes?: number;
}
