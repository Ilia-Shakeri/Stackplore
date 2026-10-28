import type * as Derived from './derived/types.derived';

export type ApplicationSettingsResponse = Derived.ApplicationSettingsResponse;
export type ApplicationSettingsValues = Derived.ApplicationValues;
export type ApplicationSettingsPreview = Derived.ApplicationPreviewResponse;
export type ConfigurationExport = Derived.ConfigurationExport;
export type CleanupPreview = Derived.CleanupPreviewResponse;
export type CleanupResult = Derived.CleanupResponse;
export type HistoryCleanupClass = Derived.HistoryClass;
export type HistoryCleanupImpact = Derived.HistoryImpact;
export type OperationPreview = Derived.OperationPreviewResponse;
export type OperationReceipt = Derived.OperationReceipt;
export type OperationResponse = Derived.OperationResponse;
export type OperationPage = Derived.OperationPage;
export type OperationAction = Derived.OperationAction;
export type OperationParameters = Derived.OperationParameters;

export type {
  Alert,
  AlertId,
  AlertRuleId,
  AlertRuleInput,
  AlertRulePage,
  AlertRuleRecord,
  AlertState,
  AuditActor,
  AuditActorKind,
  AuditEntry,
  AuditEntryId,
  AuditOutcome,
  AuditTarget,
  AvailabilityState,
  Capability,
  CapabilityName,
  CapabilitySet,
  Code,
  Completeness,
  Cursor,
  Environment,
  EnvironmentId,
  EventId,
  EventSeverity,
  EvidenceCertainty,
  EvidenceId,
  EvidenceStatus,
  HealthState,
  InfrastructureEvent,
  InfrastructureSource,
  InvalidParameter,
  LogEntry,
  LogEntryId,
  LogSeverity,
  LogStream,
  MetricName,
  MetricSample,
  MetricSampleId,
  MetricSampleSet,
  MetricUnit,
  NamespacedCode,
  NotificationId,
  NotificationSettingsStatus,
  NotificationTestResponse,
  SourceSettingsRecord,
  SourceSettingsPage,
  SourceUpdatePreview,
  SourceUpdateRequest,
  SourcePreviewRequest,
  ObservationStatus,
  PageInfo,
  Problem,
  ProductAlert,
  ProductAlertPage,
  ProductAuditPage,
  ProductEnvironment,
  ProductEnvironmentPage,
  ProductResource,
  ResourceMetadata,
  DockerResourceMetadata,
  DockerPortMetadata,
  DockerMountMetadata,
  DockerLabelMetadata,
  KubernetesResourceMetadata,
  KubernetesContainerMetadata,
  KubernetesConditionMetadata,
  ProductEvent,
  ProductEventPage,
  ProductLogPage,
  ProductLogRecord,
  ProductMetricResult,
  ProductMetricHistoryResult,
  ProductHealthHistoryResult,
  ProductHealthHistoryItem,
  ProductMetricSample,
  ProductNotification,
  ProductNotificationPage,
  ProductRelationship,
  ProductRelationshipPage,
  ProductResourceHealth,
  ProductResourceKind,
  ProductResourcePage,
  ProviderKind,
  ReadinessState,
  RelationshipDirection,
  RelationshipEvidence,
  RelationshipId,
  RelationshipKind,
  RelationshipProvenance,
  RequestId,
  Resource,
  ResourceId,
  ResourceIdentity,
  ResourceSnapshot,
  Scope,
  ScopeKind,
  SortDirection,
  SourceId,
  SourceSummary,
  SystemReadiness,
  Timestamp,
} from './derived/types.derived';

export type HealthEvidence = Derived.HealthEvidence;

export type KnownHealthSnapshot = Omit<Derived.KnownHealthSnapshot, 'evidence'> & {
  evidence: [HealthEvidence, ...HealthEvidence[]];
};

export type DataAllowedUnknownHealthSnapshot = Derived.DataAllowedUnknownHealthSnapshot;

export type NoDataUnknownHealthSnapshot = Omit<Derived.NoDataUnknownHealthSnapshot, 'evidence'> & {
  evidence: [];
};

export type UnknownHealthSnapshot = DataAllowedUnknownHealthSnapshot | NoDataUnknownHealthSnapshot;

export type HealthSnapshot = KnownHealthSnapshot | UnknownHealthSnapshot;

export type DataAllowedHealthResult = Omit<Derived.DataAllowedHealthResult, 'items'> & {
  items: HealthSnapshot[];
};

export type NoDataHealthResult = Omit<Derived.NoDataHealthResult, 'items'> & {
  items: [];
};

export type HealthResult = DataAllowedHealthResult | NoDataHealthResult;

export type Relationship = Omit<Derived.Relationship, 'evidence'> & {
  evidence: [Derived.RelationshipEvidence, ...Derived.RelationshipEvidence[]];
};

export type DataAllowedResourcePage = Derived.DataAllowedResourcePage;
export type NoDataResourcePage = Omit<Derived.NoDataResourcePage, 'items'> & { items: [] };
export type ResourcePage = DataAllowedResourcePage | NoDataResourcePage;

export type DataAllowedEnvironmentPage = Derived.DataAllowedEnvironmentPage;
export type NoDataEnvironmentPage = Omit<Derived.NoDataEnvironmentPage, 'items'> & { items: [] };
export type EnvironmentPage = DataAllowedEnvironmentPage | NoDataEnvironmentPage;

export type DataAllowedRelationshipPage = Omit<Derived.DataAllowedRelationshipPage, 'items'> & {
  items: Relationship[];
};
export type NoDataRelationshipPage = Omit<Derived.NoDataRelationshipPage, 'items'> & { items: [] };
export type RelationshipPage = DataAllowedRelationshipPage | NoDataRelationshipPage;

export type DataAllowedLogBatch = Derived.DataAllowedLogBatch;
export type NoDataLogBatch = Omit<Derived.NoDataLogBatch, 'items'> & { items: [] };
export type LogBatch = DataAllowedLogBatch | NoDataLogBatch;

export type EmptyMetricSampleSet = Derived.EmptyMetricSampleSet;
export type DataAllowedCurrentMetricSet = Derived.DataAllowedCurrentMetricSet;
export type NoDataCurrentMetricSet = Derived.NoDataCurrentMetricSet;
export type CurrentMetricSet = DataAllowedCurrentMetricSet | NoDataCurrentMetricSet;

export type DataAllowedCurrentMetricResult = Omit<
  Derived.DataAllowedCurrentMetricResult,
  'items'
> & {
  items: CurrentMetricSet[];
};
export type NoDataCurrentMetricResult = Omit<Derived.NoDataCurrentMetricResult, 'items'> & {
  items: [];
};
export type CurrentMetricResult = DataAllowedCurrentMetricResult | NoDataCurrentMetricResult;

export type DataAllowedEventPage = Derived.DataAllowedEventPage;
export type NoDataEventPage = Omit<Derived.NoDataEventPage, 'items'> & { items: [] };
export type EventPage = DataAllowedEventPage | NoDataEventPage;

export type {
  AssistantActiveRun,
  AssistantAnswerText,
  AssistantCitationId,
  AssistantCompletedRun,
  AssistantConversationId,
  AssistantEventCursor,
  AssistantLocale,
  AssistantLostResponse,
  AssistantOtherSection,
  AssistantPendingResponse,
  AssistantResetEvent,
  AssistantResourceCitation,
  AssistantRun,
  AssistantRunId,
  AssistantRunReceipt,
  AssistantRunRequest,
  AssistantRunScope,
  AssistantRunState,
  AssistantStatusEvent,
  AssistantTimeRange,
  AssistantUnavailableResponse,
  AssistantWindowMinutes,
} from './derived/types.derived';

export type AssistantFactSection = Omit<Derived.AssistantFactSection, 'citations'> & {
  citations: [Derived.AssistantCitationId, ...Derived.AssistantCitationId[]];
};

export type AssistantAnswerSection = AssistantFactSection | Derived.AssistantOtherSection;

export type AssistantAnswer = Omit<Derived.AssistantAnswer, 'sections'> & {
  sections: [AssistantAnswerSection, ...AssistantAnswerSection[]];
};

export type AssistantReadyResponse = Omit<
  Derived.AssistantReadyResponse,
  'answer' | 'citations'
> & {
  answer: AssistantAnswer;
  citations: [Derived.AssistantResourceCitation];
};

export type AssistantRunResponse =
  | Derived.AssistantPendingResponse
  | Derived.AssistantLostResponse
  | Derived.AssistantUnavailableResponse
  | AssistantReadyResponse;
