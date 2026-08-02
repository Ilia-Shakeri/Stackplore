export type ClientOptions = {
  baseUrl: `${string}://${string}/api/v1` | (string & {});
};

/**
 * Stable opaque local user identifier.
 */
export type UserId = string;

/**
 * Trimmed local login name compared by its lowercase ASCII canonical form.
 */
export type LoginName = string;

/**
 * Input-only local passphrase of 12 to 128 UTF-8 bytes. The server and browser enforce the byte minimum; the schema maxLength is a separate safe transport character ceiling. The value is neither trimmed nor normalized.
 */
export type Passphrase = string;

/**
 * Short-lived opaque proof kept only in browser memory.
 */
export type Proof = string;

/**
 * Installation-wide version 1 role.
 */
export type UserRole = 'administrator' | 'viewer';

/**
 * Safe local user state with no credential or session material.
 */
export type User = {
  id: UserId;
  login_name: LoginName;
  display_name: DisplayText;
  role: UserRole;
  enabled: boolean;
  created_at: Timestamp;
  updated_at: Timestamp;
  /**
   * Optimistic concurrency version used to build the quoted If-Match value.
   */
  version: number;
};

/**
 * Permanently closed or currently available first-run network setup state.
 */
export type SetupState = 'required' | 'closed';

/**
 * Minimum public state needed to choose setup or sign-in.
 */
export type SetupStatus = {
  state: SetupState;
  bootstrap_proof_required: boolean;
};

/**
 * Exact operation that may consume one pre-auth proof.
 */
export type PreauthPurpose = 'login' | 'setup';

/**
 * Request for a bounded pre-auth proof.
 */
export type PreauthRequest = {
  purpose: PreauthPurpose;
};

/**
 * One-use proof returned separately from its HttpOnly cookie.
 */
export type PreauthResponse = {
  preauth_proof: Proof;
  expires_at: Timestamp;
};

/**
 * First-administrator fields plus the optional operator bootstrap proof.
 */
export type SetupRequest = {
  login_name: LoginName;
  display_name: DisplayText;
  password: Passphrase;
  /**
   * Operator-supplied one-use proof required only when setup status says so.
   */
  bootstrap_proof?: Proof;
};

/**
 * Local sign-in fields with no setup or role input.
 */
export type LoginRequest = {
  login_name: LoginName;
  password: Passphrase;
};

/**
 * Safe current-user state plus a session-bound CSRF proof with no session identifier.
 */
export type SessionContext = {
  user: User;
  csrf_proof: Proof;
  idle_expires_at: Timestamp;
  absolute_expires_at: Timestamp;
};

/**
 * Exact Phase 7 high-risk change bound to a one-use proof.
 */
export type RecentAuthPurpose =
  | 'account.password_change'
  | 'user.create'
  | 'user.update'
  | 'user.delete'
  | 'user.password_reset'
  | 'user.sessions_revoke';

/**
 * Current-password proof request for one exact high-risk purpose.
 */
export type RecentAuthRequest = {
  current_password: Passphrase;
  purpose: RecentAuthPurpose;
};

/**
 * One-use proof bound to the current user, session, and purpose.
 */
export type RecentAuthResponse = {
  recent_auth_proof: Proof;
  purpose: RecentAuthPurpose;
  expires_at: Timestamp;
};

/**
 * Input-only new password for the current user.
 */
export type OwnPassphraseChangeRequest = {
  new_password: Passphrase;
};

/**
 * Fields for one new enabled local user.
 */
export type CreateUserRequest = {
  login_name: LoginName;
  display_name: DisplayText;
  role: UserRole;
  password: Passphrase;
};

/**
 * One or more reviewed local user fields.
 */
export type UpdateUserRequest = {
  login_name?: LoginName;
  display_name?: DisplayText;
  role?: UserRole;
  enabled?: boolean;
};

/**
 * Input-only replacement password for one managed local user.
 */
export type ResetPassphraseRequest = {
  new_password: Passphrase;
};

/**
 * Bounded administrator-only user page with no total-count disclosure.
 */
export type UserPage = {
  items: Array<User>;
  page: PageInfo;
};

/**
 * Current bounded readiness state.
 */
export type ReadinessState = 'ready' | 'not_ready';

export type ConfigurationReadinessCheck = {
  name: 'configuration';
  state: ReadinessState;
};

export type DatabaseReadinessCheck = {
  name: 'database';
  state: ReadinessState;
};

export type ServiceReadinessCheck = {
  name: 'service';
  state: ReadinessState;
};

/**
 * Administrator-only detailed readiness with a fixed check order.
 */
export type SystemReadiness = {
  state: ReadinessState;
  checks: [ConfigurationReadinessCheck, DatabaseReadinessCheck, ServiceReadinessCheck];
};

/**
 * Stable bounded machine code. Consumers must handle unknown valid codes.
 */
export type Code = string;

/**
 * Stable bounded domain code with at least two lowercase segments.
 */
export type NamespacedCode = string;

/**
 * Safe request correlation identifier.
 */
export type RequestId = string;

/**
 * UTC RFC 3339 timestamp.
 */
export type Timestamp = string;

/**
 * Opaque server-owned continuation cursor.
 */
export type Cursor = string;

/**
 * Trimmed non-empty display text with a visible base character.
 */
export type DisplayText = string;

/**
 * Empty or trimmed display text with a visible base character.
 */
export type OptionalDisplayText = string;

/**
 * Stable opaque infrastructure source identifier.
 */
export type SourceId = string;

/**
 * Stable opaque environment identifier.
 */
export type EnvironmentId = string;

/**
 * Stable opaque resource identifier.
 */
export type ResourceId = string;

/**
 * Stable opaque relationship identifier.
 */
export type RelationshipId = string;

/**
 * Stable opaque evidence identifier.
 */
export type EvidenceId = string;

/**
 * Stable opaque log entry identifier within its source evidence.
 */
export type LogEntryId = string;

/**
 * Stable opaque metric sample identifier.
 */
export type MetricSampleId = string;

/**
 * Stable opaque infrastructure event identifier.
 */
export type EventId = string;

/**
 * Stable opaque alert rule identifier.
 */
export type AlertRuleId = string;

/**
 * Stable opaque alert identifier.
 */
export type AlertId = string;

/**
 * Stable opaque audit entry identifier.
 */
export type AuditEntryId = string;

/**
 * Supported infrastructure provider family.
 */
export type ProviderKind = 'demo' | 'docker' | 'kubernetes';

/**
 * Normalized provider scope family.
 */
export type ScopeKind = 'global' | 'project' | 'namespace';

/**
 * Current support and access state, separate from resource health.
 */
export type AvailabilityState =
  'available' | 'stale' | 'unavailable' | 'unsupported' | 'permission_denied';

/**
 * Normalized resource health state.
 */
export type HealthState = 'healthy' | 'degraded' | 'unhealthy' | 'unknown';

/**
 * Result carried by one bounded item of health evidence.
 */
export type EvidenceStatus = 'positive' | 'negative' | 'unknown';

/**
 * Direction represented by a relationship.
 */
export type RelationshipDirection = 'source_to_target' | 'bidirectional' | 'undirected';

/**
 * Normalized relationship meaning shared by all providers.
 */
export type RelationshipKind =
  'owns' | 'selects' | 'routes_to' | 'depends_on' | 'mounts' | 'references' | 'member_of';

/**
 * Reviewed source of relationship evidence.
 */
export type RelationshipProvenance =
  | 'ownership'
  | 'selector'
  | 'route'
  | 'annotation'
  | 'declared_dependency'
  | 'storage_reference'
  | 'network_membership'
  | 'endpoint';

/**
 * Whether evidence is explicit or inferred.
 */
export type EvidenceCertainty = 'explicit' | 'inferred';

/**
 * Fixed normalized metric name for version 1.
 */
export type MetricName =
  | 'cpu.usage.cores'
  | 'memory.usage.bytes'
  | 'network.receive.bytes_total'
  | 'network.send.bytes_total'
  | 'restart.count'
  | 'cpu.request.cores'
  | 'cpu.limit.cores'
  | 'memory.request.bytes'
  | 'memory.limit.bytes';

/**
 * Normalized unit for a metric value.
 */
export type MetricUnit = 'cores' | 'bytes' | 'count';

/**
 * Fixed observation capability set for version 1.
 */
export type CapabilityName =
  | 'inventory.list'
  | 'inventory.detail'
  | 'health.read'
  | 'topology.read'
  | 'log.read'
  | 'log.tail'
  | 'metric.current'
  | 'event.history'
  | 'event.watch';

/**
 * Provider stream that supplied a log entry.
 */
export type LogStream = 'stdout' | 'stderr' | 'system' | 'unknown';

/**
 * Detected log severity when one can be determined.
 */
export type LogSeverity = 'trace' | 'debug' | 'info' | 'warning' | 'error' | 'critical' | 'unknown';

/**
 * Normalized infrastructure event severity.
 */
export type EventSeverity = 'debug' | 'info' | 'warning' | 'error' | 'critical';

/**
 * Alert lifecycle state.
 */
export type AlertState = 'pending' | 'firing' | 'acknowledged' | 'resolved';

/**
 * Kind of principal responsible for an audited event.
 */
export type AuditActorKind = 'user' | 'system' | 'operator';

/**
 * Safe normalized audit outcome.
 */
export type AuditOutcome = 'intent' | 'success' | 'failure' | 'denied' | 'uncertain';

/**
 * Direction for one operation-owned sort field.
 */
export type SortDirection = 'asc' | 'desc';

/**
 * Whether a bounded provider read returned every allowed result.
 */
export type Completeness = 'complete' | 'partial';

/**
 * Complete provider data that is currently available.
 */
export type AvailableCompleteObservation = {
  state: 'available';
  completeness: 'complete';
  observed_at: Timestamp;
  reason_code?: NamespacedCode;
};

/**
 * Partial provider data that is currently available.
 */
export type AvailablePartialObservation = {
  state: 'available';
  completeness: 'partial';
  observed_at: Timestamp;
  reason_code: NamespacedCode;
};

/**
 * Complete or partial provider data that is no longer current.
 */
export type StaleObservation = {
  state: 'stale';
  completeness: Completeness;
  observed_at: Timestamp;
  reason_code: NamespacedCode;
};

/**
 * Provider data that cannot be returned for a known safe reason.
 */
export type UnavailableObservation = {
  state: 'unavailable' | 'unsupported' | 'permission_denied';
  completeness: 'complete';
  observed_at: Timestamp;
  reason_code: NamespacedCode;
};

/**
 * Valid availability and completeness state for one bounded provider read.
 */
export type ObservationStatus =
  | AvailableCompleteObservation
  | AvailablePartialObservation
  | StaleObservation
  | UnavailableObservation;

/**
 * Safe non-secret configured infrastructure source.
 */
export type InfrastructureSource = {
  id: SourceId;
  provider: ProviderKind;
  /**
   * Human-selected safe source name.
   */
  display_name: DisplayText;
  /**
   * Whether all source evidence is synthetic demonstration data.
   */
  synthetic: boolean;
};

/**
 * Provider-neutral infrastructure environment.
 */
export type Environment = {
  id: EnvironmentId;
  source_id: SourceId;
  provider: ProviderKind;
  /**
   * Safe display name that is not part of identity.
   */
  display_name: DisplayText;
  status: ObservationStatus;
  /**
   * Whether the environment comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Provider-neutral global scope with no identity name.
 */
export type GlobalScope = {
  kind: 'global';
  name: '';
};

/**
 * Provider-neutral project or namespace scope.
 */
export type NamedScope = {
  kind: 'project' | 'namespace';
  /**
   * Safe project or namespace identity name.
   */
  name: DisplayText;
};

/**
 * Valid provider-neutral namespace, project, or global scope.
 */
export type Scope = GlobalScope | NamedScope;

/**
 * Safe public identity for a provider-neutral resource.
 */
export type ResourceIdentity = {
  id: ResourceId;
  provider: ProviderKind;
  source_id: SourceId;
  environment_id: EnvironmentId;
  scope: Scope;
  kind: NamespacedCode;
};

/**
 * Provider-neutral resource with no raw provider payload.
 */
export type Resource = {
  identity: ResourceIdentity;
  /**
   * Safe display name that is not part of identity.
   */
  display_name: DisplayText;
  /**
   * Whether the resource comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * One resource and the availability of its observed state.
 */
export type ResourceSnapshot = {
  resource: Resource;
  status: ObservationStatus;
};

/**
 * One bounded, safe fact used to calculate health.
 */
export type HealthEvidence = {
  id: EvidenceId;
  code: NamespacedCode;
  source_code: NamespacedCode;
  status: EvidenceStatus;
  observed_at: Timestamp;
  /**
   * Whether the evidence comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Known health backed by evidence from readable provider data.
 */
export type KnownHealthSnapshot = {
  resource_id: ResourceId;
  state: 'healthy' | 'degraded' | 'unhealthy';
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
  /**
   * Reviewed evidence only; raw provider objects are forbidden.
   */
  evidence: Array<HealthEvidence>;
};

/**
 * Unknown health from readable provider data, with optional reviewed evidence.
 */
export type DataAllowedUnknownHealthSnapshot = {
  resource_id: ResourceId;
  state: 'unknown';
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
  /**
   * Reviewed evidence only; raw provider objects are forbidden.
   */
  evidence: Array<HealthEvidence>;
};

/**
 * Unknown health when provider data is unavailable, unsupported, or denied.
 */
export type NoDataUnknownHealthSnapshot = {
  resource_id: ResourceId;
  state: 'unknown';
  status: UnavailableObservation;
  /**
   * Empty because the provider state does not allow data.
   */
  evidence: Array<HealthEvidence>;
};

/**
 * Unknown health split by whether the provider state allows data.
 */
export type UnknownHealthSnapshot = DataAllowedUnknownHealthSnapshot | NoDataUnknownHealthSnapshot;

/**
 * Health at one observed point, separate from data availability.
 */
export type HealthSnapshot = KnownHealthSnapshot | UnknownHealthSnapshot;

/**
 * Bounded health snapshots from a data-allowed provider read.
 */
export type DataAllowedHealthResult = {
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
  items: Array<HealthSnapshot>;
};

/**
 * Empty health result when provider data is not allowed.
 */
export type NoDataHealthResult = {
  status: UnavailableObservation;
  items: Array<HealthSnapshot>;
};

/**
 * Health result split by whether the provider status allows data.
 */
export type HealthResult = DataAllowedHealthResult | NoDataHealthResult;

/**
 * Provenance and confidence for one relationship fact.
 */
export type RelationshipEvidence = {
  id: EvidenceId;
  provider: ProviderKind;
  source_id: SourceId;
  provenance: RelationshipProvenance;
  certainty: EvidenceCertainty;
  /**
   * Normalized confidence from zero through one.
   */
  confidence: number;
  observed_at: Timestamp;
  /**
   * Whether the evidence comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Typed edge retaining all bounded evidence.
 */
export type Relationship = {
  id: RelationshipId;
  source_resource_id: ResourceId;
  target_resource_id: ResourceId;
  type: RelationshipKind;
  direction: RelationshipDirection;
  /**
   * Unique evidence retained when equivalent edges are coalesced.
   */
  evidence: Array<RelationshipEvidence>;
  observed_at: Timestamp;
};

/**
 * Page state with a required opaque continuation cursor.
 */
export type MorePageInfo = {
  /**
   * Applied page limit.
   */
  limit: number;
  has_more: true;
  next_cursor: Cursor;
};

/**
 * Final page state with no continuation cursor.
 */
export type FinalPageInfo = {
  /**
   * Applied page limit.
   */
  limit: number;
  has_more: false;
};

/**
 * Valid opaque cursor pagination state with no implied total count.
 */
export type PageInfo = MorePageInfo | FinalPageInfo;

/**
 * Bounded resource page from a data-allowed provider read.
 */
export type DataAllowedResourcePage = {
  /**
   * Resources in requested sort order with ascending resource ID as the stable tie-break.
   */
  items: Array<ResourceSnapshot>;
  page: PageInfo;
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
};

/**
 * Empty final resource page when provider data is not allowed.
 */
export type NoDataResourcePage = {
  items: Array<ResourceSnapshot>;
  page: FinalPageInfo;
  status: UnavailableObservation;
};

/**
 * Resource page split by whether the provider status allows data.
 */
export type ResourcePage = DataAllowedResourcePage | NoDataResourcePage;

/**
 * Bounded environment page from a data-allowed provider read.
 */
export type DataAllowedEnvironmentPage = {
  /**
   * Environments ordered by environment ID ascending.
   */
  items: Array<Environment>;
  page: PageInfo;
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
};

/**
 * Empty final environment page when provider data is not allowed.
 */
export type NoDataEnvironmentPage = {
  items: Array<Environment>;
  page: FinalPageInfo;
  status: UnavailableObservation;
};

/**
 * Environment page split by whether the provider status allows data.
 */
export type EnvironmentPage = DataAllowedEnvironmentPage | NoDataEnvironmentPage;

/**
 * Bounded relationship page from a data-allowed provider read.
 */
export type DataAllowedRelationshipPage = {
  /**
   * Relationships ordered by relationship ID ascending.
   */
  items: Array<Relationship>;
  page: PageInfo;
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
};

/**
 * Empty final relationship page when provider data is not allowed.
 */
export type NoDataRelationshipPage = {
  items: Array<Relationship>;
  page: FinalPageInfo;
  status: UnavailableObservation;
};

/**
 * Relationship page split by whether the provider status allows data.
 */
export type RelationshipPage = DataAllowedRelationshipPage | NoDataRelationshipPage;

/**
 * Bounded, redaction-aware provider log entry.
 */
export type LogEntry = {
  id: LogEntryId;
  resource_id: ResourceId;
  /**
   * Safe container name when the resource has multiple log targets.
   */
  container?: OptionalDisplayText;
  timestamp: Timestamp;
  /**
   * Monotonic order within one bounded read or stream scope.
   */
  sequence: number;
  stream: LogStream;
  severity: LogSeverity;
  /**
   * Safe one-line log text after configured redaction and a 64 KiB byte cap.
   */
  message: string;
  /**
   * Whether the source line exceeded the safe retained byte bound.
   */
  truncated: boolean;
  /**
   * Whether one or more values were removed.
   */
  redacted: boolean;
  /**
   * Whether the entry comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Bounded log page from a data-allowed provider read.
 */
export type DataAllowedLogBatch = {
  /**
   * Log entries ordered by sequence ascending.
   */
  items: Array<LogEntry>;
  page: PageInfo;
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
};

/**
 * Empty final log page when provider data is not allowed.
 */
export type NoDataLogBatch = {
  items: Array<LogEntry>;
  page: FinalPageInfo;
  status: UnavailableObservation;
};

/**
 * Log page split by whether the provider status allows data.
 */
export type LogBatch = DataAllowedLogBatch | NoDataLogBatch;

export type MetricSample = (
  | {
      name: 'cpu.usage.cores';
      unit: 'cores';
      value?: number;
    }
  | {
      name: 'memory.usage.bytes';
      unit: 'bytes';
      value?: number;
    }
  | {
      name: 'network.receive.bytes_total';
      unit: 'bytes';
      value?: number;
    }
  | {
      name: 'network.send.bytes_total';
      unit: 'bytes';
      value?: number;
    }
  | {
      name: 'restart.count';
      unit: 'count';
      value?: number;
    }
  | {
      name: 'cpu.request.cores';
      unit: 'cores';
      value?: number;
    }
  | {
      name: 'cpu.limit.cores';
      unit: 'cores';
      value?: number;
    }
  | {
      name: 'memory.request.bytes';
      unit: 'bytes';
      value?: number;
    }
  | {
      name: 'memory.limit.bytes';
      unit: 'bytes';
      value?: number;
    }
) & {
  id: MetricSampleId;
  resource_id: ResourceId;
  name: MetricName;
  unit: MetricUnit;
  /**
   * Finite normalized value. Non-finite values are invalid JSON.
   */
  value: number;
  sampled_at: Timestamp;
  /**
   * Observation interval when the metric needs one.
   */
  interval_seconds?: number;
  /**
   * Whether the sample comes from demonstration data.
   */
  synthetic: boolean;
};

export type CpuUsageMetricSample = MetricSample & {
  name: 'cpu.usage.cores';
  unit: 'cores';
};

export type MemoryUsageMetricSample = MetricSample & {
  name: 'memory.usage.bytes';
  unit: 'bytes';
};

export type NetworkReceiveMetricSample = MetricSample & {
  name: 'network.receive.bytes_total';
  unit: 'bytes';
};

export type NetworkSendMetricSample = MetricSample & {
  name: 'network.send.bytes_total';
  unit: 'bytes';
};

export type RestartCountMetricSample = MetricSample & {
  name: 'restart.count';
  unit: 'count';
};

export type CpuRequestMetricSample = MetricSample & {
  name: 'cpu.request.cores';
  unit: 'cores';
};

export type CpuLimitMetricSample = MetricSample & {
  name: 'cpu.limit.cores';
  unit: 'cores';
};

export type MemoryRequestMetricSample = MetricSample & {
  name: 'memory.request.bytes';
  unit: 'bytes';
};

export type MemoryLimitMetricSample = MetricSample & {
  name: 'memory.limit.bytes';
  unit: 'bytes';
};

/**
 * Unique current metric samples keyed by their fixed normalized names.
 */
export type MetricSampleSet = {
  'cpu.usage.cores'?: CpuUsageMetricSample;
  'memory.usage.bytes'?: MemoryUsageMetricSample;
  'network.receive.bytes_total'?: NetworkReceiveMetricSample;
  'network.send.bytes_total'?: NetworkSendMetricSample;
  'restart.count'?: RestartCountMetricSample;
  'cpu.request.cores'?: CpuRequestMetricSample;
  'cpu.limit.cores'?: CpuLimitMetricSample;
  'memory.request.bytes'?: MemoryRequestMetricSample;
  'memory.limit.bytes'?: MemoryLimitMetricSample;
};

/**
 * Bounded current metric samples from a data-allowed resource read.
 */
export type DataAllowedCurrentMetricSet = {
  resource_id: ResourceId;
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
  samples: MetricSampleSet;
};

/**
 * Empty metric sample set used when provider data is not allowed.
 */
export type EmptyMetricSampleSet = {
  [key: string]: never;
};

/**
 * Empty current metric set when resource data is not allowed.
 */
export type NoDataCurrentMetricSet = {
  resource_id: ResourceId;
  status: UnavailableObservation;
  samples: EmptyMetricSampleSet;
};

/**
 * Resource metric set split by whether its status allows data.
 */
export type CurrentMetricSet = DataAllowedCurrentMetricSet | NoDataCurrentMetricSet;

/**
 * Bounded current metrics from a data-allowed provider read.
 */
export type DataAllowedCurrentMetricResult = {
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
  items: Array<CurrentMetricSet>;
};

/**
 * Empty current metric result when provider data is not allowed.
 */
export type NoDataCurrentMetricResult = {
  status: UnavailableObservation;
  items: Array<CurrentMetricSet>;
};

/**
 * Current metric result split by whether the provider status allows data.
 */
export type CurrentMetricResult = DataAllowedCurrentMetricResult | NoDataCurrentMetricResult;

/**
 * Safe provider-neutral infrastructure timeline event.
 */
export type InfrastructureEvent = {
  id: EventId;
  source_id: SourceId;
  environment_id?: EnvironmentId;
  resource_id?: ResourceId;
  provider: ProviderKind;
  kind: NamespacedCode;
  severity: EventSeverity;
  occurred_at: Timestamp;
  observed_at: Timestamp;
  summary_code: NamespacedCode;
  /**
   * Bounded event summary after safe filtering.
   */
  message?: OptionalDisplayText;
  /**
   * Whether the event comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Bounded event page from a data-allowed provider read.
 */
export type DataAllowedEventPage = {
  /**
   * Events ordered by occurrence time descending, then event ID ascending.
   */
  items: Array<InfrastructureEvent>;
  page: PageInfo;
  status: AvailableCompleteObservation | AvailablePartialObservation | StaleObservation;
};

/**
 * Empty final event page when provider data is not allowed.
 */
export type NoDataEventPage = {
  items: Array<InfrastructureEvent>;
  page: FinalPageInfo;
  status: UnavailableObservation;
};

/**
 * Event page split by whether the provider status allows data.
 */
export type EventPage = DataAllowedEventPage | NoDataEventPage;

/**
 * Pending or firing alert with no lifecycle completion time.
 */
export type ActiveAlert = {
  id: AlertId;
  rule_id: AlertRuleId;
  source_id: SourceId;
  environment_id?: EnvironmentId;
  resource_id?: ResourceId;
  state: 'pending' | 'firing';
  severity: EventSeverity;
  started_at: Timestamp;
  updated_at: Timestamp;
  evidence_from: Timestamp;
  evidence_to: Timestamp;
  summary_code: NamespacedCode;
  /**
   * Whether the alert comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Acknowledged alert with no resolution time.
 */
export type AcknowledgedAlert = {
  id: AlertId;
  rule_id: AlertRuleId;
  source_id: SourceId;
  environment_id?: EnvironmentId;
  resource_id?: ResourceId;
  state: 'acknowledged';
  severity: EventSeverity;
  started_at: Timestamp;
  updated_at: Timestamp;
  acknowledged_at: Timestamp;
  evidence_from: Timestamp;
  evidence_to: Timestamp;
  summary_code: NamespacedCode;
  /**
   * Whether the alert comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Resolved alert with an optional prior acknowledgement.
 */
export type ResolvedAlert = {
  id: AlertId;
  rule_id: AlertRuleId;
  source_id: SourceId;
  environment_id?: EnvironmentId;
  resource_id?: ResourceId;
  state: 'resolved';
  severity: EventSeverity;
  started_at: Timestamp;
  updated_at: Timestamp;
  acknowledged_at?: Timestamp;
  resolved_at: Timestamp;
  evidence_from: Timestamp;
  evidence_to: Timestamp;
  summary_code: NamespacedCode;
  /**
   * Whether the alert comes from demonstration data.
   */
  synthetic: boolean;
};

/**
 * Valid provider-neutral alert lifecycle state and evidence window.
 */
export type Alert = ActiveAlert | AcknowledgedAlert | ResolvedAlert;

/**
 * Safe audited user reference with no session data.
 */
export type UserAuditActor = {
  kind: 'user';
  /**
   * Opaque user identifier.
   */
  id: string;
};

/**
 * Safe local operator reference with no session data.
 */
export type OperatorAuditActor = {
  kind: 'operator';
  /**
   * Opaque operator identifier when one exists.
   */
  id?: string;
};

/**
 * System actor with no human identifier.
 */
export type SystemAuditActor = {
  kind: 'system';
};

/**
 * Valid safe actor reference with no session data.
 */
export type AuditActor = UserAuditActor | OperatorAuditActor | SystemAuditActor;

/**
 * Safe typed target reference.
 */
export type AuditTarget = {
  kind: NamespacedCode;
  /**
   * Opaque target identifier when one exists.
   */
  id?: string;
};

/**
 * Safe audit fact with no credential or session material.
 */
export type AuditEntry = {
  id: AuditEntryId;
  occurred_at: Timestamp;
  actor: AuditActor;
  action: NamespacedCode;
  target: AuditTarget;
  outcome: AuditOutcome;
  request_id: RequestId;
  summary_code: NamespacedCode;
};

/**
 * Current inventory list capability status.
 */
export type InventoryListCapability = {
  name: 'inventory.list';
  status: ObservationStatus;
};

/**
 * Current inventory detail capability status.
 */
export type InventoryDetailCapability = {
  name: 'inventory.detail';
  status: ObservationStatus;
};

/**
 * Current health read capability status.
 */
export type HealthReadCapability = {
  name: 'health.read';
  status: ObservationStatus;
};

/**
 * Current topology read capability status.
 */
export type TopologyReadCapability = {
  name: 'topology.read';
  status: ObservationStatus;
};

/**
 * Current log read capability status.
 */
export type LogReadCapability = {
  name: 'log.read';
  status: ObservationStatus;
};

/**
 * Current log tail capability status.
 */
export type LogTailCapability = {
  name: 'log.tail';
  status: ObservationStatus;
};

/**
 * Current metric read capability status.
 */
export type MetricCurrentCapability = {
  name: 'metric.current';
  status: ObservationStatus;
};

/**
 * Current event history capability status.
 */
export type EventHistoryCapability = {
  name: 'event.history';
  status: ObservationStatus;
};

/**
 * Current event watch capability status.
 */
export type EventWatchCapability = {
  name: 'event.watch';
  status: ObservationStatus;
};

/**
 * Current status of one known observation capability.
 */
export type Capability =
  | InventoryListCapability
  | InventoryDetailCapability
  | HealthReadCapability
  | TopologyReadCapability
  | LogReadCapability
  | LogTailCapability
  | MetricCurrentCapability
  | EventHistoryCapability
  | EventWatchCapability;

/**
 * Bounded observation capabilities for one source.
 */
export type CapabilitySet = {
  provider: ProviderKind;
  source_id: SourceId;
  items: [
    InventoryListCapability,
    InventoryDetailCapability,
    HealthReadCapability,
    TopologyReadCapability,
    LogReadCapability,
    LogTailCapability,
    MetricCurrentCapability,
    EventHistoryCapability,
    EventWatchCapability,
  ];
};

/**
 * Safe field failure that never repeats a rejected value.
 */
export type InvalidParameter = {
  /**
   * Request section containing the invalid field.
   */
  location: 'query' | 'path' | 'header' | 'body';
  /**
   * Bounded field name or JSON pointer.
   */
  name: DisplayText;
  code: Code;
};

/**
 * Safe consistent failure body.
 */
export type Problem = {
  /**
   * Stable problem type URI with no request data.
   */
  type: string;
  /**
   * Fixed safe summary for the problem class.
   */
  title: DisplayText;
  /**
   * HTTP status repeated for safe parsing.
   */
  status: number;
  code: Code;
  request_id: RequestId;
  /**
   * Optional fixed safe detail with no rejected value.
   */
  detail?: OptionalDisplayText;
  invalid_parameters?: Array<InvalidParameter>;
};

/**
 * Optional safe caller correlation identifier. Invalid values are replaced.
 */
export type RequestId2 = RequestId;

/**
 * Stable opaque local user identifier.
 */
export type UserId2 = UserId;

/**
 * Strong quoted decimal form of the last observed user version.
 */
export type IfMatch = string;

/**
 * Opaque cursor bound to normalized filter and sort state; its issuer owns forward progress.
 */
export type Cursor2 = Cursor;

/**
 * Requested result count, capped by the server.
 */
export type Limit = number;

/**
 * Bounded plain-text search term, never a query expression.
 */
export type Search = OptionalDisplayText;

/**
 * Exact installation-wide role filter.
 */
export type UserRoleFilter = UserRole;

/**
 * Exact enabled-state filter.
 */
export type UserEnabledFilter = boolean;

/**
 * Repeated values use OR semantics.
 */
export type ProviderFilter = Array<ProviderKind>;

/**
 * Repeated values use OR semantics.
 */
export type SourceFilter = Array<SourceId>;

/**
 * Repeated values use OR semantics.
 */
export type EnvironmentFilter = Array<EnvironmentId>;

/**
 * Repeated bounded resource kind codes use OR semantics.
 */
export type ResourceKindFilter = Array<NamespacedCode>;

/**
 * Repeated values use OR semantics.
 */
export type HealthFilter = Array<HealthState>;

/**
 * Repeated values use OR semantics.
 */
export type AvailabilityFilter = Array<AvailabilityState>;

/**
 * Inclusive UTC start of a bounded time range.
 */
export type From = Timestamp;

/**
 * Exclusive UTC end of a bounded time range.
 */
export type To = Timestamp;

/**
 * Direction for the operation-owned sort field.
 */
export type SortDirection2 = SortDirection;

export type GetSetupStatusData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/setup/status';
};

export type GetSetupStatusErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type GetSetupStatusError = GetSetupStatusErrors[keyof GetSetupStatusErrors];

export type GetSetupStatusResponses = {
  /**
   * Current bounded network setup state.
   */
  200: SetupStatus;
};

export type GetSetupStatusResponse = GetSetupStatusResponses[keyof GetSetupStatusResponses];

export type CreateFirstAdministratorData = {
  body: SetupRequest;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/setup';
};

export type CreateFirstAdministratorErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The request conflicts with the current stable state.
   */
  409: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type CreateFirstAdministratorError =
  CreateFirstAdministratorErrors[keyof CreateFirstAdministratorErrors];

export type CreateFirstAdministratorResponses = {
  /**
   * Setup completed and a new local session was issued.
   */
  201: SessionContext;
};

export type CreateFirstAdministratorResponse =
  CreateFirstAdministratorResponses[keyof CreateFirstAdministratorResponses];

export type CreatePreauthProofData = {
  body: PreauthRequest;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/auth/preflight';
};

export type CreatePreauthProofErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The request conflicts with the current stable state.
   */
  409: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type CreatePreauthProofError = CreatePreauthProofErrors[keyof CreatePreauthProofErrors];

export type CreatePreauthProofResponses = {
  /**
   * A bounded pre-auth proof was issued.
   */
  201: PreauthResponse;
};

export type CreatePreauthProofResponse =
  CreatePreauthProofResponses[keyof CreatePreauthProofResponses];

export type LoginData = {
  body: LoginRequest;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/auth/login';
};

export type LoginErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * Local sign-in failed without disclosing account existence or state.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type LoginError = LoginErrors[keyof LoginErrors];

export type LoginResponses = {
  /**
   * A new local session was issued.
   */
  200: SessionContext;
};

export type LoginResponse = LoginResponses[keyof LoginResponses];

export type GetCurrentSessionData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/auth/session';
};

export type GetCurrentSessionErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type GetCurrentSessionError = GetCurrentSessionErrors[keyof GetCurrentSessionErrors];

export type GetCurrentSessionResponses = {
  /**
   * Current safe session context with no session identifier.
   */
  200: SessionContext;
};

export type GetCurrentSessionResponse =
  GetCurrentSessionResponses[keyof GetCurrentSessionResponses];

export type LogoutData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/auth/logout';
};

export type LogoutErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type LogoutError = LogoutErrors[keyof LogoutErrors];

export type LogoutResponses = {
  /**
   * Current session revoked and its browser cookie cleared.
   */
  204: void;
};

export type LogoutResponse = LogoutResponses[keyof LogoutResponses];

export type CreateRecentAuthGrantData = {
  body: RecentAuthRequest;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/auth/recent-grants';
};

export type CreateRecentAuthGrantErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type CreateRecentAuthGrantError =
  CreateRecentAuthGrantErrors[keyof CreateRecentAuthGrantErrors];

export type CreateRecentAuthGrantResponses = {
  /**
   * A one-use recent-authentication proof was issued.
   */
  201: RecentAuthResponse;
};

export type CreateRecentAuthGrantResponse =
  CreateRecentAuthGrantResponses[keyof CreateRecentAuthGrantResponses];

export type ChangeOwnPasswordData = {
  body: OwnPassphraseChangeRequest;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/auth/password';
};

export type ChangeOwnPasswordErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type ChangeOwnPasswordError = ChangeOwnPasswordErrors[keyof ChangeOwnPasswordErrors];

export type ChangeOwnPasswordResponses = {
  /**
   * Password changed and the current session rotated.
   */
  200: SessionContext;
};

export type ChangeOwnPasswordResponse =
  ChangeOwnPasswordResponses[keyof ChangeOwnPasswordResponses];

export type ListUsersData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: {
    /**
     * Opaque cursor bound to normalized filter and sort state; its issuer owns forward progress.
     */
    cursor?: Cursor;
    /**
     * Requested result count, capped by the server.
     */
    limit?: number;
    /**
     * Bounded plain-text search term, never a query expression.
     */
    q?: OptionalDisplayText;
    /**
     * Exact installation-wide role filter.
     */
    role?: UserRole;
    /**
     * Exact enabled-state filter.
     */
    enabled?: boolean;
  };
  url: '/users';
};

export type ListUsersErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type ListUsersError = ListUsersErrors[keyof ListUsersErrors];

export type ListUsersResponses = {
  /**
   * Bounded page of local users in canonical login order.
   */
  200: UserPage;
};

export type ListUsersResponse = ListUsersResponses[keyof ListUsersResponses];

export type CreateUserData = {
  body: CreateUserRequest;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/users';
};

export type CreateUserErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The request conflicts with the current stable state.
   */
  409: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type CreateUserError = CreateUserErrors[keyof CreateUserErrors];

export type CreateUserResponses = {
  /**
   * Local user created.
   */
  201: User;
};

export type CreateUserResponse = CreateUserResponses[keyof CreateUserResponses];

export type DeleteUserData = {
  body?: never;
  headers: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
    /**
     * Strong quoted decimal form of the last observed user version.
     */
    'If-Match': string;
  };
  path: {
    /**
     * Stable opaque local user identifier.
     */
    user_id: UserId;
  };
  query?: never;
  url: '/users/{user_id}';
};

export type DeleteUserErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The requested safe identifier does not exist.
   */
  404: Problem;
  /**
   * The request conflicts with the current stable state.
   */
  409: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type DeleteUserError = DeleteUserErrors[keyof DeleteUserErrors];

export type DeleteUserResponses = {
  /**
   * The authenticated local state change completed.
   */
  204: void;
};

export type DeleteUserResponse = DeleteUserResponses[keyof DeleteUserResponses];

export type GetUserData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path: {
    /**
     * Stable opaque local user identifier.
     */
    user_id: UserId;
  };
  query?: never;
  url: '/users/{user_id}';
};

export type GetUserErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The requested safe identifier does not exist.
   */
  404: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type GetUserError = GetUserErrors[keyof GetUserErrors];

export type GetUserResponses = {
  /**
   * Current safe local user state.
   */
  200: User;
};

export type GetUserResponse = GetUserResponses[keyof GetUserResponses];

export type UpdateUserData = {
  body: UpdateUserRequest;
  headers: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
    /**
     * Strong quoted decimal form of the last observed user version.
     */
    'If-Match': string;
  };
  path: {
    /**
     * Stable opaque local user identifier.
     */
    user_id: UserId;
  };
  query?: never;
  url: '/users/{user_id}';
};

export type UpdateUserErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The requested safe identifier does not exist.
   */
  404: Problem;
  /**
   * The request conflicts with the current stable state.
   */
  409: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type UpdateUserError = UpdateUserErrors[keyof UpdateUserErrors];

export type UpdateUserResponses = {
  /**
   * Current safe local user state.
   */
  200: User;
};

export type UpdateUserResponse = UpdateUserResponses[keyof UpdateUserResponses];

export type ResetUserPasswordData = {
  body: ResetPassphraseRequest;
  headers: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
    /**
     * Strong quoted decimal form of the last observed user version.
     */
    'If-Match': string;
  };
  path: {
    /**
     * Stable opaque local user identifier.
     */
    user_id: UserId;
  };
  query?: never;
  url: '/users/{user_id}/password-reset';
};

export type ResetUserPasswordErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The requested safe identifier does not exist.
   */
  404: Problem;
  /**
   * The request conflicts with the current stable state.
   */
  409: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * The request body media type is not supported.
   */
  415: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type ResetUserPasswordError = ResetUserPasswordErrors[keyof ResetUserPasswordErrors];

export type ResetUserPasswordResponses = {
  /**
   * The authenticated local state change completed.
   */
  204: void;
};

export type ResetUserPasswordResponse =
  ResetUserPasswordResponses[keyof ResetUserPasswordResponses];

export type RevokeUserSessionsData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path: {
    /**
     * Stable opaque local user identifier.
     */
    user_id: UserId;
  };
  query?: never;
  url: '/users/{user_id}/sessions';
};

export type RevokeUserSessionsErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The requested safe identifier does not exist.
   */
  404: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type RevokeUserSessionsError = RevokeUserSessionsErrors[keyof RevokeUserSessionsErrors];

export type RevokeUserSessionsResponses = {
  /**
   * The authenticated local state change completed.
   */
  204: void;
};

export type RevokeUserSessionsResponse =
  RevokeUserSessionsResponses[keyof RevokeUserSessionsResponses];

export type GetSystemReadinessData = {
  body?: never;
  headers?: {
    /**
     * Optional safe caller correlation identifier. Invalid values are replaced.
     */
    'X-Request-ID'?: RequestId;
  };
  path?: never;
  query?: never;
  url: '/system/readiness';
};

export type GetSystemReadinessErrors = {
  /**
   * The request is malformed or fails strict validation.
   */
  400: Problem;
  /**
   * A valid local session is required.
   */
  401: Problem;
  /**
   * The current principal cannot perform the request.
   */
  403: Problem;
  /**
   * The bounded request body limit was exceeded.
   */
  413: Problem;
  /**
   * A bounded request rate was exceeded.
   */
  429: Problem;
  /**
   * The request failed without exposing internal detail.
   */
  500: Problem;
  /**
   * The service cannot complete the request now.
   */
  503: Problem;
};

export type GetSystemReadinessError = GetSystemReadinessErrors[keyof GetSystemReadinessErrors];

export type GetSystemReadinessResponses = {
  /**
   * Detailed bounded service readiness.
   */
  200: SystemReadiness;
};

export type GetSystemReadinessResponse =
  GetSystemReadinessResponses[keyof GetSystemReadinessResponses];
