import { spawnSync } from 'node:child_process';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const contractPath = join(repositoryRoot, 'contracts', 'openapi.yaml');
const configPath = join(repositoryRoot, 'redocly.yaml');
const redoclyPath = join(repositoryRoot, 'node_modules', '@redocly', 'cli', 'bin', 'cli.js');

const fail = (message) => {
  throw new Error(`OpenAPI policy failed: ${message}`);
};

const runRedocly = (args, capture = false) => {
  const result = spawnSync(process.execPath, [redoclyPath, ...args], {
    cwd: repositoryRoot,
    encoding: 'utf8',
    env: {
      ...process.env,
      REDOCLY_TELEMETRY: 'off',
    },
    stdio: capture ? 'pipe' : 'inherit',
  });

  if (result.error) {
    throw new Error(`Could not run contract lint: ${result.error.message}`);
  }

  if (result.status !== 0) {
    if (capture) {
      process.stderr.write(result.stderr);
      process.stderr.write(result.stdout);
    }
    throw new Error(`Contract lint exited with status ${String(result.status)}.`);
  }
};

const isRecord = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

const maximumArrayItems = 200;
const maximumStringLength = 65_536;
const canonicalCursorPattern =
  '^(?:[A-Za-z0-9_-]{4})*(?:[A-Za-z0-9_-][AQgw]|[A-Za-z0-9_-]{2}[AEIMQUYcgkosw048])?$';
const canonicalTimestampPattern =
  '^(?!0000)[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\\.[0-9]{1,9})?Z$';
const resourceIdPattern = '^r1_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$';
const relationshipIdPattern = '^rel1_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$';
const displayTextPattern =
  '^(?=\\P{White_Space})(?!.*\\p{White_Space}$)(?=.*[\\p{L}\\p{N}\\p{P}\\p{S}])(?:[^\\p{Cc}\\p{Cf}\\p{Zl}\\p{Zp}]|\\u200C|\\u200D)+$';
const optionalDisplayTextPattern =
  '^(?:$|(?=\\P{White_Space})(?!.*\\p{White_Space}$)(?=.*[\\p{L}\\p{N}\\p{P}\\p{S}])(?:[^\\p{Cc}\\p{Cf}\\p{Zl}\\p{Zp}]|\\u200C|\\u200D)+)$';
const logMessagePattern = '^(?:[^\\p{Cc}\\p{Cf}\\p{Zl}\\p{Zp}]|\\u200C|\\u200D)*$';
const allowedDocumentKeys = [
  'components',
  'info',
  'jsonSchemaDialect',
  'openapi',
  'paths',
  'servers',
];
const browserSession = ['BrowserSession'];
const sessionMutation = ['BrowserSession', 'CSRFRequestProof'];
const highRiskMutation = ['BrowserSession', 'CSRFRequestProof', 'RecentAuthProof'];
const preauthMutation = ['PreauthBrowserProof', 'PreauthRequestProof'];
const guestReadPaths = new Set([
  '/sources',
  '/environments',
  '/resources',
  '/resources/{resource_id}',
  '/resources/{resource_id}/health',
  '/topology',
  '/logs',
  '/metrics/current',
  '/metrics/history',
  '/events',
  '/alerts',
  '/notifications',
  '/streams/health',
  '/streams/metrics',
  '/streams/events',
  '/streams/logs',
  '/streams/alerts',
]);
const guestReadDescription =
  'Public only in an explicitly isolated guest demonstration; normal installations require a browser session.';
const routePolicies = new Map([
  [
    '/setup/status',
    {
      get: {
        operationId: 'getSetupStatus',
        requestBody: false,
        responses: ['200', '400', '413', '429', '500', '503'],
        security: [],
      },
    },
  ],
  [
    '/setup',
    {
      post: {
        operationId: 'createFirstAdministrator',
        requestBody: true,
        responses: ['201', '400', '403', '409', '413', '415', '429', '500', '503'],
        security: preauthMutation,
      },
    },
  ],
  [
    '/auth/preflight',
    {
      post: {
        operationId: 'createPreauthProof',
        requestBody: true,
        responses: ['201', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
        security: [],
      },
    },
  ],
  [
    '/auth/login',
    {
      post: {
        operationId: 'login',
        requestBody: true,
        responses: ['200', '400', '401', '403', '413', '415', '429', '500', '503'],
        security: preauthMutation,
      },
    },
  ],
  [
    '/auth/session',
    {
      get: {
        operationId: 'getCurrentSession',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/auth/logout',
    {
      post: {
        operationId: 'logout',
        requestBody: false,
        responses: ['204', '400', '401', '403', '413', '429', '500', '503'],
        security: sessionMutation,
      },
    },
  ],
  [
    '/auth/recent-grants',
    {
      post: {
        operationId: 'createRecentAuthGrant',
        requestBody: true,
        responses: ['201', '400', '401', '403', '413', '415', '429', '500', '503'],
        security: sessionMutation,
      },
    },
  ],
  [
    '/auth/password',
    {
      patch: {
        operationId: 'changeOwnPassword',
        requestBody: true,
        responses: ['200', '400', '401', '403', '413', '415', '429', '500', '503'],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/users',
    {
      get: {
        operationId: 'listUsers',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
      post: {
        operationId: 'createUser',
        requestBody: true,
        responses: ['201', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/users/{user_id}',
    {
      delete: {
        operationId: 'deleteUser',
        requestBody: false,
        responses: ['204', '400', '401', '403', '404', '409', '413', '429', '500', '503'],
        security: highRiskMutation,
      },
      get: {
        operationId: 'getUser',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
      patch: {
        operationId: 'updateUser',
        requestBody: true,
        responses: ['200', '400', '401', '403', '404', '409', '413', '415', '429', '500', '503'],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/users/{user_id}/password-reset',
    {
      post: {
        operationId: 'resetUserPassword',
        requestBody: true,
        responses: ['204', '400', '401', '403', '404', '409', '413', '415', '429', '500', '503'],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/users/{user_id}/sessions',
    {
      delete: {
        operationId: 'revokeUserSessions',
        requestBody: false,
        responses: ['204', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/sources',
    {
      get: {
        operationId: 'listSources',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/environments',
    {
      get: {
        operationId: 'listEnvironments',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/resources',
    {
      get: {
        operationId: 'listResources',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/resources/{resource_id}',
    {
      get: {
        operationId: 'getResource',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/resources/{resource_id}/health',
    {
      get: {
        operationId: 'getResourceHealth',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/topology',
    {
      get: {
        operationId: 'getProductTopology',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/logs',
    {
      get: {
        operationId: 'listProductLogs',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/metrics/current',
    {
      get: {
        operationId: 'getProductMetrics',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/metrics/history',
    {
      get: {
        operationId: 'getProductMetricHistory',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/events',
    {
      get: {
        operationId: 'listProductEvents',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/alerts',
    {
      get: {
        operationId: 'listProductAlerts',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/alerts/{alert_id}/acknowledge',
    {
      post: {
        operationId: 'acknowledgeProductAlert',
        requestBody: false,
        responses: ['200', '400', '401', '403', '404', '409', '413', '429', '500', '503'],
        security: sessionMutation,
      },
    },
  ],
  [
    '/alert-rules',
    {
      get: {
        operationId: 'listAlertRules',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
      post: {
        operationId: 'createAlertRule',
        requestBody: true,
        responses: ['201', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
        security: sessionMutation,
      },
    },
  ],
  [
    '/alert-rules/{rule_id}',
    {
      delete: {
        operationId: 'deleteAlertRule',
        requestBody: false,
        responses: ['204', '400', '401', '403', '404', '409', '413', '429', '500', '503'],
        security: sessionMutation,
      },
      put: {
        operationId: 'replaceAlertRule',
        requestBody: true,
        responses: ['200', '400', '401', '403', '404', '409', '413', '415', '429', '500', '503'],
        security: sessionMutation,
      },
    },
  ],
  [
    '/notifications',
    {
      get: {
        operationId: 'listNotifications',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/settings/notifications',
    {
      get: {
        operationId: 'getNotificationSettings',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/settings/notifications/test',
    {
      post: {
        operationId: 'testNotificationDelivery',
        requestBody: false,
        responses: ['202', '400', '401', '403', '413', '429', '500', '503'],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/audit',
    {
      get: {
        operationId: 'listAuditEntries',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/settings/sources',
    {
      get: {
        operationId: 'listSourceSettings',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/settings/sources/{source_id}/preview',
    {
      post: {
        operationId: 'previewSourceUpdate',
        requestBody: true,
        responses: [
          '200',
          '400',
          '401',
          '403',
          '404',
          '409',
          '413',
          '415',
          '422',
          '429',
          '500',
          '503',
        ],
        security: sessionMutation,
      },
    },
  ],
  [
    '/settings/sources/{source_id}',
    {
      patch: {
        operationId: 'updateSourceSettings',
        requestBody: true,
        responses: [
          '200',
          '400',
          '401',
          '403',
          '404',
          '409',
          '413',
          '415',
          '422',
          '429',
          '500',
          '503',
        ],
        security: highRiskMutation,
      },
    },
  ],
  [
    '/streams/health',
    {
      get: {
        operationId: 'streamProductHealth',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/streams/metrics',
    {
      get: {
        operationId: 'streamProductMetrics',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/streams/events',
    {
      get: {
        operationId: 'streamProductEvents',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/streams/logs',
    {
      get: {
        operationId: 'streamProductLogs',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/streams/alerts',
    {
      get: {
        operationId: 'streamProductAlerts',
        requestBody: false,
        responses: ['200', '400', '401', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
  [
    '/system/readiness',
    {
      get: {
        operationId: 'getSystemReadiness',
        requestBody: false,
        responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
        security: browserSession,
      },
    },
  ],
]);
routePolicies.set('/settings/application', {
  get: {
    operationId: 'getApplicationSettings',
    requestBody: false,
    responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
    security: browserSession,
  },
  put: {
    operationId: 'updateApplicationSettings',
    requestBody: true,
    responses: ['200', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
    security: highRiskMutation,
  },
});
routePolicies.set('/settings/application/previews', {
  post: {
    operationId: 'previewApplicationSettings',
    requestBody: true,
    responses: ['200', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
    security: sessionMutation,
  },
});
routePolicies.set('/settings/cleanup/previews', {
  post: {
    operationId: 'previewHistoryCleanup',
    requestBody: true,
    responses: ['200', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
    security: sessionMutation,
  },
});
routePolicies.set('/settings/cleanup', {
  post: {
    operationId: 'cleanupHistory',
    requestBody: true,
    responses: ['200', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
    security: highRiskMutation,
  },
});
routePolicies.set('/settings/export', {
  post: {
    operationId: 'exportConfiguration',
    requestBody: true,
    responses: ['200', '400', '401', '403', '409', '413', '415', '429', '500', '503'],
    security: highRiskMutation,
  },
});
routePolicies.set('/operations/previews', {
  post: {
    operationId: 'previewOperation',
    requestBody: true,
    responses: ['200', '400', '401', '403', '404', '409', '413', '415', '429', '500', '503'],
    security: sessionMutation,
  },
});
routePolicies.set('/operations', {
  post: {
    operationId: 'executeOperation',
    requestBody: true,
    responses: ['200', '202', '400', '401', '403', '404', '409', '413', '415', '429', '500', '503'],
    security: highRiskMutation,
  },
  get: {
    operationId: 'listOperations',
    requestBody: false,
    responses: ['200', '400', '401', '403', '413', '429', '500', '503'],
    security: browserSession,
  },
});
routePolicies.set('/operations/{operation_id}', {
  get: {
    operationId: 'getOperation',
    requestBody: false,
    responses: ['200', '400', '401', '403', '404', '413', '429', '500', '503'],
    security: browserSession,
  },
});
const allowedPathNames = [...routePolicies.keys()];
const allowedOperationKeys = [
  'description',
  'operationId',
  'parameters',
  'responses',
  'security',
  'summary',
];
const allowedComponentCollections = [
  'examples',
  'headers',
  'parameters',
  'responses',
  'schemas',
  'securitySchemes',
];
const allowedExampleNames = ['AdministratorSession'];
const allowedHeaderNames = ['BrowserStateUpdate', 'EntityTag', 'RequestId', 'RetryAfter'];
const allowedParameterNames = [
  'AvailabilityFilter',
  'Cursor',
  'EnvironmentFilter',
  'From',
  'HealthFilter',
  'IfMatch',
  'LastEventId',
  'NotificationTestAdapter',
  'Limit',
  'ProviderFilter',
  'ProductResourceFilter',
  'ProductSourceSelection',
  'RequestId',
  'ResourceId',
  'ResourceKindFilter',
  'Search',
  'SortDirection',
  'SourceFilter',
  'SourceSettingsId',
  'To',
  'UserEnabledFilter',
  'UserCursor',
  'UserId',
  'UserRoleFilter',
];
const allowedResponseNames = [
  'BadRequestProblem',
  'ConflictProblem',
  'ForbiddenProblem',
  'InternalProblem',
  'NotFoundProblem',
  'PayloadTooLargeProblem',
  'ProductEventStream',
  'ServiceUnavailableProblem',
  'SignInRejectedProblem',
  'TooManyRequestsProblem',
  'UnauthorizedProblem',
  'UnsupportedMediaTypeProblem',
  'MutationAccepted',
  'UserCreated',
  'UserFound',
];
const retryableResponseNames = new Set(['ServiceUnavailableProblem', 'TooManyRequestsProblem']);
const successResponseNames = new Set([
  'MutationAccepted',
  'ProductEventStream',
  'UserCreated',
  'UserFound',
]);
const allowedResponseKeys = ['content', 'description', 'headers'];
const problemMediaType = 'application/problem+json';
const allowedSchemaNames = [
  'ResourceMetadata',
  'DockerMetadataEnvelope',
  'KubernetesMetadataEnvelope',
  'DockerResourceMetadata',
  'DockerPortMetadata',
  'DockerMountMetadata',
  'DockerLabelMetadata',
  'KubernetesResourceMetadata',
  'KubernetesContainerMetadata',
  'KubernetesConditionMetadata',
  'MatrixNotificationSettingsStatus',
  'ConfiguredMatrixNotificationSettingsStatus',
  'DisabledMatrixNotificationSettingsStatus',
  'ProductAlertChange',
  'ProductAlertChangesEnvelope',
  'ProductAlertSnapshotEnvelope',
  'AcknowledgedAlert',
  'ActiveAlert',
  'Alert',
  'AlertId',
  'AlertRuleId',
  'AlertRuleInput',
  'AlertRulePage',
  'AlertRuleRecord',
  'AlertState',
  'AuditActor',
  'AuditActorKind',
  'AuditEntry',
  'AuditEntryId',
  'AuditOutcome',
  'AuditTarget',
  'AvailabilityState',
  'AvailableCompleteObservation',
  'AvailablePartialObservation',
  'Capability',
  'CapabilityName',
  'CapabilitySet',
  'ConfigurationReadinessCheck',
  'Code',
  'Completeness',
  'CpuLimitMetricSample',
  'CpuRequestMetricSample',
  'CpuUsageMetricSample',
  'CurrentMetricResult',
  'CurrentMetricSet',
  'CreateUserRequest',
  'Cursor',
  'DataAllowedCurrentMetricResult',
  'DataAllowedCurrentMetricSet',
  'DataAllowedEnvironmentPage',
  'DataAllowedEventPage',
  'DataAllowedHealthResult',
  'DataAllowedLogBatch',
  'DataAllowedRelationshipPage',
  'DataAllowedResourcePage',
  'DataAllowedUnknownHealthSnapshot',
  'DatabaseReadinessCheck',
  'DisplayText',
  'EmptyMetricSampleSet',
  'Environment',
  'EnvironmentId',
  'EnvironmentPage',
  'EventHistoryCapability',
  'EventId',
  'EventPage',
  'EventSeverity',
  'EventWatchCapability',
  'EvidenceCertainty',
  'EvidenceId',
  'EvidenceStatus',
  'FinalPageInfo',
  'GlobalScope',
  'HealthEvidence',
  'HealthReadCapability',
  'HealthResult',
  'HealthSnapshot',
  'HealthState',
  'HealthSummary',
  'InfrastructureEvent',
  'InfrastructureSource',
  'InventoryDetailCapability',
  'InventoryListCapability',
  'InvalidParameter',
  'KnownHealthSnapshot',
  'LoginName',
  'LoginRequest',
  'LogBatch',
  'LogEntry',
  'LogEntryId',
  'LogReadCapability',
  'LogSeverity',
  'LogStream',
  'LogTailCapability',
  'MemoryLimitMetricSample',
  'MemoryRequestMetricSample',
  'MemoryUsageMetricSample',
  'MetricCurrentCapability',
  'MetricHistoryCapability',
  'MetricName',
  'MetricSample',
  'MetricSampleId',
  'MetricSampleSet',
  'MetricUnit',
  'MorePageInfo',
  'NamedScope',
  'NamespacedCode',
  'NetworkReceiveMetricSample',
  'NetworkSendMetricSample',
  'NoDataCurrentMetricResult',
  'NoDataCurrentMetricSet',
  'NoDataEnvironmentPage',
  'NoDataEventPage',
  'NoDataHealthResult',
  'NoDataLogBatch',
  'NoDataRelationshipPage',
  'NoDataResourcePage',
  'NoDataUnknownHealthSnapshot',
  'NotificationId',
  'NotificationSettingsStatus',
  'NotificationTestResponse',
  'ConfiguredNotificationSettingsStatus',
  'DisabledNotificationSettingsStatus',
  'ObservationStatus',
  'OperatorAuditActor',
  'OptionalDisplayText',
  'PageInfo',
  'Passphrase',
  'OwnPassphraseChangeRequest',
  'PreauthPurpose',
  'PreauthRequest',
  'PreauthResponse',
  'Problem',
  'ProductResource',
  'ProductEnvironment',
  'ProductEnvironmentPage',
  'ProductResourceHealth',
  'ProductResourceKind',
  'ProductResourcePage',
  'ProductEvent',
  'ProductEventPage',
  'ProductAlert',
  'ProductAlertPage',
  'ProductAuditPage',
  'ProductLogPage',
  'ProductLogRecord',
  'ProductMetricResult',
  'ProductMetricHistoryResult',
  'ProductMetricSample',
  'ProductNotification',
  'ProductNotificationPage',
  'ProductRelationship',
  'ProductRelationshipPage',
  'Proof',
  'ProviderKind',
  'Relationship',
  'RelationshipDirection',
  'RelationshipEvidence',
  'RelationshipId',
  'RelationshipKind',
  'RelationshipPage',
  'RelationshipProvenance',
  'RequestId',
  'ResetPassphraseRequest',
  'ReadinessState',
  'RecentAuthPurpose',
  'RecentAuthRequest',
  'RecentAuthResponse',
  'ResolvedAlert',
  'Resource',
  'ResourceId',
  'ResourceIdentity',
  'ResourcePage',
  'ResourceSnapshot',
  'RestartCountMetricSample',
  'Scope',
  'ScopeKind',
  'ServiceReadinessCheck',
  'SessionContext',
  'SetupRequest',
  'SetupState',
  'SetupStatus',
  'NormalSetupStatus',
  'GuestDemoStatus',
  'SortDirection',
  'SourceId',
  'SourceSettingsRecord',
  'SourceSettingsPage',
  'SourceUpdatePreview',
  'SourceUpdateRequest',
  'SourcePreviewRequest',
  'SourceSummary',
  'StaleObservation',
  'SystemAuditActor',
  'SystemReadiness',
  'Timestamp',
  'TopologyReadCapability',
  'UnavailableObservation',
  'UnknownHealthSnapshot',
  'UpdateUserRequest',
  'User',
  'UserAuditActor',
  'UserId',
  'UserPage',
  'UserRole',
];
const allowedSecuritySchemeNames = [
  'BrowserSession',
  'CSRFRequestProof',
  'PreauthBrowserProof',
  'PreauthRequestProof',
  'RecentAuthProof',
];
allowedSchemaNames.push(
  ...[
    'ApplicationValues',
    'ApplicationMutationMode',
    'ApplicationAuditPolicy',
    'ApplicationSettingsResponse',
    'HistoryClass',
    'HistoryImpactClass',
    'HistoryImpact',
    'ApplicationPreviewRequest',
    'ApplicationPreviewResponse',
    'ApplicationUpdateRequest',
    'CleanupPreviewRequest',
    'CleanupPreviewResponse',
    'CleanupRequest',
    'HistoryCutoff',
    'CleanupResponse',
    'ConfigurationExportRequest',
    'ConfigurationExportSource',
    'ConfigurationExportIntegration',
    'ConfigurationExport',
    'OperationAction',
    'OperationParameters',
    'OperationPreviewRequest',
    'OperationExecuteRequest',
    'OperationPreviewData',
    'OperationPreviewResponse',
    'OperationResult',
    'OperationReceipt',
    'OperationResponse',
    'OperationPage',
  ],
);
allowedParameterNames.push('OperationId', 'OperationIdempotencyKey');
const forbiddenPropertyFragments = [
  'accesskey',
  'apikey',
  'authorization',
  'cookie',
  'credential',
  'kubeconfig',
  'metadata',
  'password',
  'passwd',
  'privatekey',
  'rawpayload',
  'secret',
  'sessionid',
  'sessiontoken',
  'token',
];
const sensitiveInputFieldAllowlist = new Set([
  'ProductResource:metadata',
  'CreateUserRequest:password',
  'LoginRequest:password',
  'OwnPassphraseChangeRequest:new_password',
  'RecentAuthRequest:current_password',
  'ResetPassphraseRequest:new_password',
  'SetupRequest:password',
  'SourceUpdateRequest:confirmation_token',
  'SourceUpdatePreview:confirmation_token',
  'ApplicationPreviewResponse:confirmation_token',
  'ApplicationUpdateRequest:confirmation_token',
  'CleanupPreviewResponse:confirmation_token',
  'CleanupRequest:confirmation_token',
  'OperationPreviewData:confirmation_token',
  'OperationExecuteRequest:confirmation_token',
  'ConfigurationExportSource:credential_reference',
  'ConfigurationExportIntegration:credential_reference',
]);
const reviewedMetadataSchemaNames = new Set([
  'ResourceMetadata',
  'DockerMetadataEnvelope',
  'KubernetesMetadataEnvelope',
  'DockerResourceMetadata',
  'DockerPortMetadata',
  'DockerMountMetadata',
  'DockerLabelMetadata',
  'KubernetesResourceMetadata',
  'KubernetesContainerMetadata',
  'KubernetesConditionMetadata',
]);
const schemaAssertionKeywords = new Set([
  '$ref',
  'additionalProperties',
  'allOf',
  'anyOf',
  'const',
  'contains',
  'dependentRequired',
  'dependentSchemas',
  'else',
  'enum',
  'exclusiveMaximum',
  'exclusiveMinimum',
  'if',
  'items',
  'maxContains',
  'maxItems',
  'maxLength',
  'maxProperties',
  'maximum',
  'minContains',
  'minItems',
  'minLength',
  'minProperties',
  'minimum',
  'multipleOf',
  'not',
  'oneOf',
  'pattern',
  'patternProperties',
  'prefixItems',
  'properties',
  'propertyNames',
  'required',
  'then',
  'type',
  'unevaluatedItems',
  'unevaluatedProperties',
  'uniqueItems',
]);

const isForbiddenPropertyName = (name) => {
  const normalized = name.toLowerCase().replaceAll(/[^a-z0-9]/gu, '');
  return (
    normalized === 'session' ||
    forbiddenPropertyFragments.some((fragment) => normalized.includes(fragment))
  );
};

const inputFieldPolicyKey = (location, name) => {
  const schemaMatch = /^components\.schemas\.([A-Za-z0-9_]+)/u.exec(location);
  if (schemaMatch === null) {
    return null;
  }
  const path = [
    ...`${location}.properties.${name}`.matchAll(/\.properties\.([A-Za-z0-9_-]+)/gu),
  ].map((match) => match[1]);
  return `${schemaMatch[1]}:${path.join('.')}`;
};

const hasSafeRootSchemaShape = (schema) =>
  isRecord(schema) &&
  (typeof schema.type === 'string' ||
    typeof schema.$ref === 'string' ||
    ['allOf', 'anyOf', 'oneOf'].some((keyword) => Array.isArray(schema[keyword])));

const hasExactKeys = (value, expected) => {
  if (!isRecord(value)) {
    return false;
  }

  const actual = Object.keys(value).sort();
  const sortedExpected = [...expected].sort();
  return (
    actual.length === sortedExpected.length &&
    actual.every((key, index) => key === sortedExpected[index])
  );
};

const inspectPathPolicy = (paths) => {
  const issues = [];
  if (!hasExactKeys(paths, allowedPathNames)) {
    return ['paths must contain only the fixed product routes'];
  }

  const operationIds = new Set();
  for (const [path, methodPolicies] of routePolicies) {
    const pathItem = paths[path];
    if (!hasExactKeys(pathItem, Object.keys(methodPolicies))) {
      issues.push(`${path} must keep its fixed methods`);
      continue;
    }

    for (const [method, policy] of Object.entries(methodPolicies)) {
      const operation = pathItem[method];
      const expectedKeys = policy.requestBody
        ? [...allowedOperationKeys, 'requestBody']
        : allowedOperationKeys;
      if (!hasExactKeys(operation, expectedKeys)) {
        issues.push(`${path} ${method.toUpperCase()} must keep its fixed operation shape`);
        continue;
      }
      if (operation.operationId !== policy.operationId || operationIds.has(operation.operationId)) {
        issues.push(`${path} ${method.toUpperCase()} must keep its unique operation ID`);
      }
      operationIds.add(operation.operationId);
      if (
        typeof operation.summary !== 'string' ||
        operation.summary.length === 0 ||
        typeof operation.description !== 'string' ||
        operation.description.length === 0
      ) {
        issues.push(`${path} ${method.toUpperCase()} must have bounded descriptive text`);
      }
      if (
        !Array.isArray(operation.parameters) ||
        operation.parameters.length === 0 ||
        operation.parameters[0]?.$ref !== '#/components/parameters/RequestId'
      ) {
        issues.push(`${path} ${method.toUpperCase()} must begin with the shared request ID`);
      }
      if (!hasExactKeys(operation.responses, policy.responses)) {
        issues.push(`${path} ${method.toUpperCase()} must keep its fixed response statuses`);
      }

      if (policy.security.length === 0) {
        if (!Array.isArray(operation.security) || operation.security.length !== 0) {
          issues.push(`${path} ${method.toUpperCase()} must be explicitly public`);
        }
      } else if (
        !Array.isArray(operation.security) ||
        operation.security.length !== (method === 'get' && guestReadPaths.has(path) ? 2 : 1) ||
        !hasExactKeys(operation.security[0], policy.security) ||
        (method === 'get' &&
          guestReadPaths.has(path) &&
          (!hasExactKeys(operation.security[1], []) ||
            !operation.description.includes(guestReadDescription))) ||
        Object.values(operation.security[0]).some(
          (scopes) => !Array.isArray(scopes) || scopes.length !== 0,
        )
      ) {
        issues.push(`${path} ${method.toUpperCase()} must keep its fixed proof set`);
      }

      if (
        policy.requestBody &&
        (operation.requestBody?.required !== true ||
          !hasExactKeys(operation.requestBody?.content, ['application/json']))
      ) {
        issues.push(`${path} ${method.toUpperCase()} must require one JSON body`);
      }
    }
  }

  return issues;
};

const inspectComponentPolicy = (components) => {
  const issues = [];
  if (!isRecord(components)) {
    return ['components must be an object'];
  }

  for (const name of Object.keys(components)) {
    if (!allowedComponentCollections.includes(name)) {
      issues.push(`components.${name} is not allowed in Phase 7`);
    }
  }

  for (const [collectionName, collection] of Object.entries(components)) {
    if (!isRecord(collection)) {
      continue;
    }
    for (const name of Object.keys(collection)) {
      if (
        isForbiddenPropertyName(name) &&
        !(collectionName === 'schemas' && reviewedMetadataSchemaNames.has(name))
      ) {
        issues.push(`components.${collectionName}.${name} uses an unsafe name`);
      }
    }
  }

  if (isRecord(components.parameters)) {
    for (const [name, parameter] of Object.entries(components.parameters)) {
      if (!isRecord(parameter)) {
        issues.push(`components.parameters.${name} must be an object`);
      } else if ('content' in parameter) {
        issues.push(`components.parameters.${name} must use schema, not content`);
      }
      if (
        isRecord(parameter) &&
        typeof parameter.name === 'string' &&
        isForbiddenPropertyName(parameter.name)
      ) {
        issues.push(`components.parameters.${name}.name uses an unsafe name`);
      }
    }
  }
  if (isRecord(components.responses)) {
    for (const [responseName, response] of Object.entries(components.responses)) {
      if (!isRecord(response)) {
        issues.push(`components.responses.${responseName} must be an object`);
        continue;
      }

      const responseLocation = `components.responses.${responseName}`;
      const extraResponseKeys = Object.keys(response).filter(
        (key) => !allowedResponseKeys.includes(key),
      );
      if (extraResponseKeys.length > 0) {
        issues.push(`${responseLocation} has unsupported response fields`);
      }

      if (typeof response.description !== 'string' || response.description.length === 0) {
        issues.push(`${responseLocation}.description must be non-empty text`);
      }

      if (!isRecord(response.headers)) {
        issues.push(`${responseLocation}.headers must be an object`);
      } else {
        const expectedHeaders =
          responseName === 'ProductEventStream'
            ? ['Cache-Control', 'Set-Cookie', 'X-Accel-Buffering', 'X-Request-ID']
            : responseName === 'UserCreated' || responseName === 'UserFound'
              ? ['ETag', 'Set-Cookie', 'X-Request-ID']
              : retryableResponseNames.has(responseName)
                ? ['Retry-After', 'Set-Cookie', 'X-Request-ID']
                : ['Set-Cookie', 'X-Request-ID'];
        const actualHeaders = Object.keys(response.headers).sort();
        if (
          actualHeaders.length !== expectedHeaders.length ||
          actualHeaders.some((headerName, index) => headerName !== expectedHeaders[index])
        ) {
          issues.push(`${responseLocation}.headers must use the fixed response header set`);
        }

        for (const headerName of actualHeaders) {
          if (headerName !== 'Set-Cookie' && isForbiddenPropertyName(headerName)) {
            issues.push(`${responseLocation}.headers.${headerName} uses an unsafe name`);
          }
        }
      }

      if (responseName === 'ProductEventStream') {
        if (
          !isRecord(response.content) ||
          !hasExactKeys(response.content, ['text/event-stream']) ||
          !isRecord(response.content['text/event-stream']) ||
          !hasExactKeys(response.content['text/event-stream'], ['schema']) ||
          !isRecord(response.content['text/event-stream'].schema) ||
          response.content['text/event-stream'].schema.type !== 'string'
        ) {
          issues.push(`${responseLocation}.content must use one documented SSE body`);
        }
        continue;
      }

      if (successResponseNames.has(responseName)) {
        if (responseName === 'MutationAccepted') {
          if ('content' in response) {
            issues.push(`${responseLocation} must not return a body`);
          }
        } else if (
          !isRecord(response.content) ||
          Object.keys(response.content).length !== 1 ||
          !isRecord(response.content['application/json']) ||
          !hasExactKeys(response.content['application/json'], ['examples', 'schema'])
        ) {
          issues.push(`${responseLocation}.content must use one documented JSON body`);
        }
        continue;
      }

      if (
        !isRecord(response.content) ||
        Object.keys(response.content).length !== 1 ||
        !isRecord(response.content[problemMediaType])
      ) {
        issues.push(`${responseLocation}.content must use only ${problemMediaType}`);
      } else {
        const mediaKeys = Object.keys(response.content[problemMediaType]);
        if (
          mediaKeys.length !== 2 ||
          !mediaKeys.includes('examples') ||
          !mediaKeys.includes('schema')
        ) {
          issues.push(
            `${responseLocation}.content.${problemMediaType} must use schema and examples`,
          );
        }
      }
    }
  }
  return issues;
};

const inspectSchema = (schema, location, issues, allowFalse = false) => {
  if (schema === true) {
    issues.push(`${location} must not permit every value`);
    return;
  }
  if (schema === false) {
    if (!allowFalse) {
      issues.push(`${location} may be false only to close a prefix tuple`);
    }
    return;
  }
  if (!isRecord(schema)) {
    issues.push(`${location} must be a schema object`);
    return;
  }

  if (Object.keys(schema).length === 0) {
    issues.push(`${location} must not be an unconstrained schema`);
    return;
  }

  if (!Object.keys(schema).some((keyword) => schemaAssertionKeywords.has(keyword))) {
    issues.push(`${location} must contain a JSON Schema assertion`);
  }

  if (
    'type' in schema &&
    (typeof schema.type !== 'string' ||
      !['array', 'boolean', 'integer', 'null', 'number', 'object', 'string'].includes(schema.type))
  ) {
    issues.push(`${location} must use one explicit JSON Schema type`);
  }

  if (schema.type === 'object' && schema.additionalProperties !== false) {
    issues.push(`${location} must set additionalProperties to false`);
  }

  if ('additionalProperties' in schema && schema.additionalProperties !== false) {
    issues.push(`${location} permits an open property map`);
  }

  if ('unevaluatedProperties' in schema && schema.unevaluatedProperties !== false) {
    issues.push(`${location} permits unevaluated properties`);
  }

  if (isRecord(schema.patternProperties) && Object.keys(schema.patternProperties).length > 0) {
    issues.push(`${location} permits a pattern property map`);
  }

  if (schema.type === 'array') {
    if (
      !Number.isSafeInteger(schema.maxItems) ||
      schema.maxItems < 0 ||
      schema.maxItems > maximumArrayItems
    ) {
      issues.push(`${location} must set maxItems no higher than ${String(maximumArrayItems)}`);
    }
    const hasItemSchema = isRecord(schema.items);
    const hasClosedPrefixTuple =
      Array.isArray(schema.prefixItems) && schema.prefixItems.length > 0 && schema.items === false;
    if (!hasItemSchema && !hasClosedPrefixTuple) {
      issues.push(`${location} must define items or a closed prefix tuple`);
    }
  }

  if (schema.type === 'string' && !Array.isArray(schema.enum) && typeof schema.const !== 'string') {
    if (
      !Number.isSafeInteger(schema.maxLength) ||
      schema.maxLength < 0 ||
      schema.maxLength > maximumStringLength
    ) {
      issues.push(`${location} must set maxLength no higher than ${String(maximumStringLength)}`);
    }
  }

  if (schema.type === 'integer') {
    if (
      !Number.isSafeInteger(schema.minimum) ||
      !Number.isSafeInteger(schema.maximum) ||
      schema.minimum < Number.MIN_SAFE_INTEGER ||
      schema.maximum > Number.MAX_SAFE_INTEGER ||
      schema.minimum > schema.maximum
    ) {
      issues.push(`${location} must set safe integer minimum and maximum bounds`);
    }
  }

  if (schema.type === 'number') {
    if (
      !Number.isFinite(schema.minimum) ||
      !Number.isFinite(schema.maximum) ||
      schema.minimum < Number.MIN_SAFE_INTEGER ||
      schema.maximum > Number.MAX_SAFE_INTEGER ||
      schema.minimum > schema.maximum
    ) {
      issues.push(`${location} must set safe numeric minimum and maximum bounds`);
    }
  }

  if (isRecord(schema.properties)) {
    for (const [name, property] of Object.entries(schema.properties)) {
      const policyKey = inputFieldPolicyKey(location, name);
      if (
        isForbiddenPropertyName(name) &&
        (policyKey === null || !sensitiveInputFieldAllowlist.has(policyKey))
      ) {
        issues.push(`${location} exposes forbidden field ${name}`);
      }
      inspectSchema(property, `${location}.properties.${name}`, issues);
    }
  }

  for (const keyword of [
    'items',
    'contains',
    'contentSchema',
    'else',
    'if',
    'not',
    'propertyNames',
    'then',
    'unevaluatedItems',
  ]) {
    if (keyword in schema) {
      const allowClosedTuple =
        (keyword === 'items' || keyword === 'unevaluatedItems') &&
        Array.isArray(schema.prefixItems) &&
        schema.prefixItems.length > 0;
      inspectSchema(schema[keyword], `${location}.${keyword}`, issues, allowClosedTuple);
    }
  }

  for (const keyword of ['$defs', 'definitions', 'dependentSchemas', 'patternProperties']) {
    const values = schema[keyword];
    if (isRecord(values)) {
      for (const [name, value] of Object.entries(values)) {
        inspectSchema(value, `${location}.${keyword}.${name}`, issues);
      }
    }
  }

  for (const keyword of ['allOf', 'anyOf', 'oneOf', 'prefixItems']) {
    const values = schema[keyword];
    if (Array.isArray(values)) {
      values.forEach((value, index) => {
        const limbLocation = `${location}.${keyword}.${String(index)}`;
        if (!isRecord(value) || Object.keys(value).length === 0) {
          issues.push(`${limbLocation} must not permit every value`);
          return;
        }
        inspectSchema(value, limbLocation, issues);
      });
    }
  }
};

const collectLocalSchemaReferences = (value, references) => {
  if (Array.isArray(value)) {
    for (const item of value) {
      collectLocalSchemaReferences(item, references);
    }
    return;
  }
  if (!isRecord(value)) {
    return;
  }

  if (typeof value.$ref === 'string') {
    const match = /^#\/components\/schemas\/([A-Za-z0-9_]+)$/u.exec(value.$ref);
    if (match !== null) {
      references.add(match[1]);
    }
  }

  for (const child of Object.values(value)) {
    collectLocalSchemaReferences(child, references);
  }
};

const findSchemaReferenceCycle = (schemas) => {
  if (!isRecord(schemas)) {
    return null;
  }

  const graph = new Map();
  for (const [name, schema] of Object.entries(schemas)) {
    const references = new Set();
    collectLocalSchemaReferences(schema, references);
    graph.set(name, references);
  }

  const states = new Map();
  const trail = [];
  const visit = (name) => {
    if (states.get(name) === 2) {
      return null;
    }
    if (states.get(name) === 1) {
      const cycleStart = trail.indexOf(name);
      return [...trail.slice(cycleStart), name];
    }

    states.set(name, 1);
    trail.push(name);
    for (const reference of graph.get(name) ?? []) {
      if (!graph.has(reference)) {
        continue;
      }
      const cycle = visit(reference);
      if (cycle !== null) {
        return cycle;
      }
    }
    trail.pop();
    states.set(name, 2);
    return null;
  };

  for (const name of graph.keys()) {
    const cycle = visit(name);
    if (cycle !== null) {
      return cycle;
    }
  }
  return null;
};

const runPolicySelfTests = () => {
  const unsafeNames = [
    'access-key',
    'api_key',
    'api_token',
    'clientSecret',
    'kube-config',
    'PASSWORD_HASH',
    'private_key_pem',
    'rawPayload',
    'session-id',
  ];
  if (unsafeNames.some((name) => !isForbiddenPropertyName(name))) {
    throw new Error('OpenAPI policy self-test failed to identify an unsafe field name.');
  }
  if (['next_cursor', 'request_id', 'source_id', 'synthetic'].some(isForbiddenPropertyName)) {
    throw new Error('OpenAPI policy self-test rejected a safe field name.');
  }

  const unsafeObject = {
    type: 'object',
    additionalProperties: false,
    properties: {
      private_key: { type: 'string', maxLength: 128 },
    },
  };
  const limbIssues = [];
  inspectSchema(
    {
      $defs: { unsafe: unsafeObject },
      if: unsafeObject,
      then: unsafeObject,
      else: unsafeObject,
      not: unsafeObject,
    },
    'components.schemas.PolicySelfTest',
    limbIssues,
  );
  if (limbIssues.filter((issue) => issue.includes('forbidden field')).length !== 5) {
    throw new Error('OpenAPI policy self-test failed to walk every schema limb.');
  }

  const boundIssues = [];
  inspectSchema(
    { type: 'array', maxItems: maximumArrayItems + 1, items: { type: 'boolean' } },
    'array',
    boundIssues,
  );
  inspectSchema({ type: 'integer', minimum: 0 }, 'integer', boundIssues);
  inspectSchema({ type: 'string', maxLength: maximumStringLength + 1 }, 'string', boundIssues);
  if (boundIssues.length !== 3) {
    throw new Error('OpenAPI policy self-test failed to enforce numeric bounds.');
  }

  const permissiveIssues = [];
  inspectSchema(
    { type: ['string', 'null'], maxLength: 8 },
    'components.schemas.NullableText',
    permissiveIssues,
  );
  inspectSchema({ anyOf: [{}] }, 'components.schemas.EmptyUnion', permissiveIssues);
  inspectSchema({ oneOf: [true] }, 'components.schemas.BooleanUnion', permissiveIssues);
  if (permissiveIssues.length !== 3) {
    throw new Error('OpenAPI policy self-test failed to reject permissive schema forms.');
  }

  const assertionIssues = [];
  inspectSchema(
    { description: 'annotation only' },
    'components.schemas.Annotation',
    assertionIssues,
  );
  inspectSchema({ type: 'array', maxItems: 1 }, 'components.schemas.MissingItems', assertionIssues);
  if (assertionIssues.length !== 2) {
    throw new Error('OpenAPI policy self-test failed to reject accept-all schema nodes.');
  }

  const booleanCases = [
    true,
    { type: 'array', maxItems: 1, items: true },
    {
      type: 'array',
      maxItems: 1,
      prefixItems: [{ type: 'string', maxLength: 1 }],
      items: false,
      unevaluatedItems: true,
    },
  ];
  if (
    booleanCases.some((schema, index) => {
      const issues = [];
      inspectSchema(schema, `components.schemas.BooleanCase${String(index)}`, issues);
      return issues.length === 0;
    })
  ) {
    throw new Error('OpenAPI policy self-test failed to reject true boolean schemas.');
  }

  const closedTupleIssues = [];
  inspectSchema(
    {
      type: 'array',
      maxItems: 1,
      prefixItems: [{ type: 'string', maxLength: 1 }],
      items: false,
    },
    'components.schemas.ClosedTuple',
    closedTupleIssues,
  );
  if (closedTupleIssues.length !== 0) {
    throw new Error('OpenAPI policy self-test rejected a closed prefix tuple.');
  }

  if (
    hasSafeRootSchemaShape({ properties: { value: { type: 'string', maxLength: 8 } } }) ||
    hasSafeRootSchemaShape({ not: { type: 'null' } }) ||
    !hasSafeRootSchemaShape({ type: 'object', additionalProperties: false }) ||
    !hasSafeRootSchemaShape({ oneOf: [{ type: 'string', maxLength: 8 }] }) ||
    !hasSafeRootSchemaShape({ $ref: '#/components/schemas/Code' })
  ) {
    throw new Error('OpenAPI policy self-test failed to enforce safe root schema shapes.');
  }

  const badComponentIssues = inspectComponentPolicy({
    callbacks: {},
    headers: {},
    links: {},
    parameters: { Unsafe: { content: {} } },
    pathItems: {},
    requestBodies: {},
    responses: {},
    schemas: {},
  });
  if (badComponentIssues.length !== 5) {
    throw new Error('OpenAPI policy self-test failed to reject unsafe component forms.');
  }

  const unsafeMemberIssues = inspectComponentPolicy({
    headers: { SessionToken: {} },
    parameters: { ApiToken: { name: 'api_token', schema: { type: 'string' } } },
    responses: {},
    schemas: {},
  });
  if (unsafeMemberIssues.length !== 3) {
    throw new Error('OpenAPI policy self-test failed to reject unsafe component names.');
  }

  const unsafeResponseShapeIssues = inspectComponentPolicy({
    responses: {
      BadRequestProblem: {
        content: {
          'application/problem+json': { schema: { $ref: '#/components/schemas/Problem' } },
          'text/plain': { schema: { type: 'string', maxLength: 8 } },
        },
        description: 'Unsafe response shape',
        headers: {
          'X-Debug': { $ref: '#/components/headers/RequestId' },
          'X-Request-ID': { $ref: '#/components/headers/RequestId' },
        },
        links: {},
      },
    },
  });
  if (
    !unsafeResponseShapeIssues.some((issue) => issue.includes('unsupported response fields')) ||
    !unsafeResponseShapeIssues.some((issue) => issue.includes('fixed response header set')) ||
    !unsafeResponseShapeIssues.some((issue) => issue.includes('must use only'))
  ) {
    throw new Error('OpenAPI policy self-test failed to reject unsafe response shapes.');
  }

  const createValidPhase7Paths = () =>
    Object.fromEntries(
      [...routePolicies].map(([path, methods]) => [
        path,
        Object.fromEntries(
          Object.entries(methods).map(([method, policy]) => {
            const operation = {
              description:
                method === 'get' && guestReadPaths.has(path)
                  ? guestReadDescription
                  : 'Bounded Phase 7 operation.',
              operationId: policy.operationId,
              parameters: [{ $ref: '#/components/parameters/RequestId' }],
              responses: Object.fromEntries(
                policy.responses.map((status) => [
                  status,
                  { $ref: '#/components/responses/BadRequestProblem' },
                ]),
              ),
              security:
                policy.security.length === 0
                  ? []
                  : [
                      Object.fromEntries(policy.security.map((name) => [name, []])),
                      ...(method === 'get' && guestReadPaths.has(path) ? [{}] : []),
                    ],
              summary: 'Bounded operation',
            };
            if (policy.requestBody) {
              operation.requestBody = {
                content: { 'application/json': {} },
                required: true,
              };
            }
            return [method, operation];
          }),
        ),
      ]),
    );
  const validPhase7Paths = createValidPhase7Paths();
  if (inspectPathPolicy(validPhase7Paths).length !== 0) {
    throw new Error('OpenAPI policy self-test rejected the fixed Phase 7 routes.');
  }

  const unsafePathMutations = [
    (paths) => {
      paths['/unexpected'] = {};
    },
    (paths) => {
      paths['/setup'].put = {};
    },
    (paths) => {
      delete paths['/setup'].post.responses[201];
    },
    (paths) => {
      paths['/system/readiness'].get.security = [];
    },
    (paths) => {
      paths['/system/readiness'].get.security.push({});
    },
    (paths) => {
      paths['/sources'].get.security = [{}];
    },
    (paths) => {
      paths['/sources'].get.description = 'Public without deployment isolation.';
    },
    (paths) => {
      delete paths['/auth/login'].post.requestBody;
    },
    (paths) => {
      paths['/users'].post.operationId = 'listUsers';
    },
  ];
  for (const mutate of unsafePathMutations) {
    const paths = createValidPhase7Paths();
    mutate(paths);
    if (inspectPathPolicy(paths).length === 0) {
      throw new Error('OpenAPI policy self-test failed to reject unsafe Phase 7 paths.');
    }
  }

  const directCycle = findSchemaReferenceCycle({
    Direct: { $ref: '#/components/schemas/Direct' },
  });
  const indirectCycle = findSchemaReferenceCycle({
    First: { $ref: '#/components/schemas/Second' },
    Second: { $ref: '#/components/schemas/First' },
  });
  const acyclic = findSchemaReferenceCycle({
    Leaf: { type: 'string', maxLength: 8 },
    Root: { $ref: '#/components/schemas/Leaf' },
  });
  if (directCycle === null || indirectCycle === null || acyclic !== null) {
    throw new Error('OpenAPI policy self-test failed to reject schema reference cycles.');
  }
};

const assertExactValues = (actual, expected, location) => {
  if (!Array.isArray(actual) || actual.length !== expected.length) {
    fail(`${location} must contain the fixed version 1 values`);
  }

  const actualValues = [...actual].sort();
  const expectedValues = [...expected].sort();
  if (actualValues.some((value, index) => value !== expectedValues[index])) {
    fail(`${location} must contain the fixed version 1 values`);
  }
};

const assertExactStringSchema = (
  schema,
  { format, maximumLength, minimumLength, pattern },
  location,
) => {
  if (
    !isRecord(schema) ||
    schema.type !== 'string' ||
    schema.maxLength !== maximumLength ||
    schema.pattern !== pattern ||
    (minimumLength === undefined ? 'minLength' in schema : schema.minLength !== minimumLength) ||
    (format === undefined ? 'format' in schema : schema.format !== format)
  ) {
    fail(`${location} must keep its exact string contract`);
  }
};

const main = async () => {
  runPolicySelfTests();
  const source = await readFile(contractPath, 'utf8');
  for (const [index, line] of source.split(/\r?\n/u).entries()) {
    if (!line.includes('$ref:')) {
      continue;
    }
    const match = /^\s*(?:-\s*)?\$ref:\s*'(#\/[A-Za-z0-9_~./-]+)'\s*$/u.exec(line);
    if (!match) {
      fail(`reference on line ${String(index + 1)} must use canonical local form`);
    }
  }

  runRedocly(['lint', contractPath, '--config', configPath, '--format', 'stylish']);

  const temporaryDirectory = await mkdtemp(join(tmpdir(), 'stackplore-openapi-policy-'));
  const bundledPath = join(temporaryDirectory, 'openapi.json');

  try {
    runRedocly(
      ['bundle', contractPath, '--config', configPath, '--ext', 'json', '--output', bundledPath],
      true,
    );
    const document = JSON.parse(await readFile(bundledPath, 'utf8'));

    if (!isRecord(document) || document.openapi !== '3.1.0') {
      fail('document must use OpenAPI 3.1.0');
    }

    if (document.jsonSchemaDialect !== 'https://json-schema.org/draft/2020-12/schema') {
      fail('document must use the JSON Schema 2020-12 dialect');
    }

    assertExactValues(Object.keys(document), allowedDocumentKeys, 'document');

    if (
      !Array.isArray(document.servers) ||
      document.servers.length !== 1 ||
      !isRecord(document.servers[0]) ||
      document.servers[0].url !== '/api/v1'
    ) {
      fail('server base must be exactly /api/v1');
    }

    const pathIssues = inspectPathPolicy(document.paths);
    if (pathIssues.length > 0) {
      fail(pathIssues.join('; '));
    }
    for (const [path, method, status] of [
      ['/auth/session', 'get', '200'],
      ['/auth/recent-grants', 'post', '201'],
      ['/users', 'get', '200'],
      ['/system/readiness', 'get', '200'],
    ]) {
      if (
        document.paths?.[path]?.[method]?.responses?.[status]?.headers?.['Set-Cookie']?.$ref !==
        '#/components/headers/BrowserStateUpdate'
      ) {
        fail(`${path} ${method.toUpperCase()} must document session cookie renewal`);
      }
    }

    if ('webhooks' in document) {
      fail('webhooks are outside the Phase 7 boundary');
    }

    const components = document.components;
    if (!isRecord(components) || !isRecord(components.schemas)) {
      fail('components.schemas is required');
    }

    const componentIssues = inspectComponentPolicy(components);
    if (componentIssues.length > 0) {
      fail(componentIssues.join('; '));
    }
    assertExactValues(Object.keys(components), allowedComponentCollections, 'components');
    assertExactValues(Object.keys(components.schemas), allowedSchemaNames, 'components.schemas');
    assertExactValues(
      Object.keys(components.parameters),
      allowedParameterNames,
      'components.parameters',
    );
    assertExactValues(Object.keys(components.headers), allowedHeaderNames, 'components.headers');
    assertExactValues(
      Object.keys(components.responses),
      allowedResponseNames,
      'components.responses',
    );
    assertExactValues(Object.keys(components.examples), allowedExampleNames, 'components.examples');
    assertExactValues(
      Object.keys(components.securitySchemes),
      allowedSecuritySchemeNames,
      'components.securitySchemes',
    );
    const expectedSecuritySchemes = new Map([
      ['BrowserSession', ['cookie', 'stackplore_session']],
      ['CSRFRequestProof', ['header', 'X-Stackplore-CSRF']],
      ['PreauthBrowserProof', ['cookie', 'stackplore_preauth']],
      ['PreauthRequestProof', ['header', 'X-Stackplore-Preauth']],
      ['RecentAuthProof', ['header', 'X-Stackplore-Recent-Auth']],
    ]);
    for (const [name, [location, wireName]] of expectedSecuritySchemes) {
      const scheme = components.securitySchemes[name];
      if (
        !isRecord(scheme) ||
        scheme.type !== 'apiKey' ||
        scheme.in !== location ||
        scheme.name !== wireName ||
        typeof scheme.description !== 'string' ||
        scheme.description.length === 0
      ) {
        fail(`components.securitySchemes.${name} must keep its exact proof contract`);
      }
    }

    const referenceCycle = findSchemaReferenceCycle(components.schemas);
    if (referenceCycle !== null) {
      fail(`schema references must be acyclic: ${referenceCycle.join(' -> ')}`);
    }

    const issues = [];
    for (const [name, schema] of Object.entries(components.schemas)) {
      if (!hasSafeRootSchemaShape(schema)) {
        issues.push(`components.schemas.${name} must declare type, local ref, or composition`);
      }
      inspectSchema(schema, `components.schemas.${name}`, issues);
    }

    for (const collectionName of ['headers', 'parameters']) {
      const collection = components[collectionName];
      if (!isRecord(collection)) {
        continue;
      }
      for (const [name, component] of Object.entries(collection)) {
        if (isRecord(component) && isRecord(component.schema)) {
          inspectSchema(component.schema, `components.${collectionName}.${name}.schema`, issues);
        }
      }
    }

    if (isRecord(components.responses)) {
      for (const [name, response] of Object.entries(components.responses)) {
        if (name === 'ProductEventStream') {
          continue;
        }
        if (!isRecord(response) || !isRecord(response.content)) {
          continue;
        }
        for (const [mediaType, media] of Object.entries(response.content)) {
          if (isRecord(media) && isRecord(media.schema)) {
            inspectSchema(
              media.schema,
              `components.responses.${name}.content.${mediaType}.schema`,
              issues,
            );
          }
        }
      }
    }

    if (issues.length > 0) {
      fail(issues.join('; '));
    }

    const fixedEnums = new Map([
      ['AlertState', ['pending', 'firing', 'acknowledged', 'resolved']],
      ['AuditActorKind', ['user', 'operator', 'system']],
      ['AuditOutcome', ['intent', 'success', 'failure', 'denied', 'uncertain']],
      [
        'AvailabilityState',
        ['available', 'stale', 'unavailable', 'unsupported', 'permission_denied'],
      ],
      [
        'CapabilityName',
        [
          'inventory.list',
          'inventory.detail',
          'health.read',
          'topology.read',
          'log.read',
          'log.tail',
          'metric.current',
          'metric.history',
          'event.history',
          'event.watch',
        ],
      ],
      ['Completeness', ['complete', 'partial']],
      ['EventSeverity', ['debug', 'info', 'warning', 'error', 'critical']],
      ['EvidenceCertainty', ['explicit', 'inferred']],
      ['EvidenceStatus', ['positive', 'negative', 'unknown']],
      ['HealthState', ['healthy', 'degraded', 'unhealthy', 'unknown']],
      ['LogSeverity', ['trace', 'debug', 'info', 'warning', 'error', 'critical', 'unknown']],
      ['LogStream', ['stdout', 'stderr', 'system', 'unknown']],
      [
        'MetricName',
        [
          'cpu.usage.cores',
          'memory.usage.bytes',
          'network.receive.bytes_total',
          'network.send.bytes_total',
          'restart.count',
          'cpu.request.cores',
          'cpu.limit.cores',
          'memory.request.bytes',
          'memory.limit.bytes',
        ],
      ],
      ['MetricUnit', ['cores', 'bytes', 'count']],
      ['ProviderKind', ['demo', 'docker', 'kubernetes']],
      ['PreauthPurpose', ['login', 'setup']],
      ['ReadinessState', ['ready', 'not_ready']],
      [
        'RecentAuthPurpose',
        [
          'account.password_change',
          'user.create',
          'user.update',
          'user.delete',
          'user.password_reset',
          'user.sessions_revoke',
          'notification.test',
          'source.update',
          'operation.execute',
          'settings.update',
          'configuration.export',
          'data.cleanup',
        ],
      ],
      ['RelationshipDirection', ['source_to_target', 'bidirectional', 'undirected']],
      [
        'RelationshipKind',
        ['owns', 'selects', 'routes_to', 'depends_on', 'mounts', 'references', 'member_of'],
      ],
      [
        'RelationshipProvenance',
        [
          'ownership',
          'selector',
          'route',
          'annotation',
          'declared_dependency',
          'storage_reference',
          'network_membership',
          'endpoint',
        ],
      ],
      ['ScopeKind', ['global', 'project', 'namespace']],
      ['SetupState', ['required', 'closed']],
      ['SortDirection', ['asc', 'desc']],
      ['UserRole', ['administrator', 'viewer']],
    ]);
    for (const [name, expected] of fixedEnums) {
      const schema = components.schemas[name];
      if (!isRecord(schema)) {
        fail(`fixed enum schema is missing: ${name}`);
      }
      assertExactValues(schema.enum, expected, name);
    }

    assertExactStringSchema(
      components.schemas.Timestamp,
      {
        format: 'date-time',
        maximumLength: 30,
        minimumLength: 20,
        pattern: canonicalTimestampPattern,
      },
      'Timestamp',
    );
    assertExactStringSchema(
      components.schemas.ResourceId,
      { maximumLength: 46, minimumLength: 46, pattern: resourceIdPattern },
      'ResourceId',
    );
    assertExactStringSchema(
      components.schemas.RelationshipId,
      { maximumLength: 48, minimumLength: 48, pattern: relationshipIdPattern },
      'RelationshipId',
    );
    assertExactStringSchema(
      components.schemas.DisplayText,
      { maximumLength: 256, minimumLength: 1, pattern: displayTextPattern },
      'DisplayText',
    );
    assertExactStringSchema(
      components.schemas.OptionalDisplayText,
      { maximumLength: 8192, pattern: optionalDisplayTextPattern },
      'OptionalDisplayText',
    );
    assertExactStringSchema(
      components.schemas.Passphrase,
      { format: 'password', maximumLength: 128 },
      'Passphrase',
    );
    for (const name of ['ResourceId', 'RelationshipId']) {
      const schema = components.schemas[name];
      if (
        !Array.isArray(schema.examples) ||
        schema.examples.length !== 1 ||
        !new RegExp(schema.pattern, 'u').test(schema.examples[0])
      ) {
        fail(`${name} example must match the canonical digest form`);
      }
    }

    const displayReferences = [
      [
        'InfrastructureSource.display_name',
        components.schemas.InfrastructureSource?.properties?.display_name,
        256,
      ],
      ['Environment.display_name', components.schemas.Environment?.properties?.display_name, 256],
      ['NamedScope.name', components.schemas.NamedScope?.properties?.name, 256],
      ['Resource.display_name', components.schemas.Resource?.properties?.display_name, 256],
      ['InvalidParameter.name', components.schemas.InvalidParameter?.properties?.name, 128],
      ['Problem.title', components.schemas.Problem?.properties?.title, 128],
    ];
    for (const [location, schema, maximumLength] of displayReferences) {
      if (
        !isRecord(schema) ||
        schema.$ref !== '#/components/schemas/DisplayText' ||
        (maximumLength === 256 ? 'maxLength' in schema : schema.maxLength !== maximumLength)
      ) {
        fail(`${location} must use bounded DisplayText`);
      }
    }
    const optionalDisplayReferences = [
      ['LogEntry.container', components.schemas.LogEntry?.properties?.container, 256],
      [
        'InfrastructureEvent.message',
        components.schemas.InfrastructureEvent?.properties?.message,
        8192,
      ],
      ['Problem.detail', components.schemas.Problem?.properties?.detail, 1024],
      ['parameters.Search', components.parameters?.Search?.schema, 200],
    ];
    for (const [location, schema, maximumLength] of optionalDisplayReferences) {
      if (
        !isRecord(schema) ||
        schema.$ref !== '#/components/schemas/OptionalDisplayText' ||
        (maximumLength === 8192 ? 'maxLength' in schema : schema.maxLength !== maximumLength)
      ) {
        fail(`${location} must use bounded OptionalDisplayText`);
      }
    }
    const logMessage = components.schemas.LogEntry?.properties?.message;
    if (
      !isRecord(logMessage) ||
      logMessage.type !== 'string' ||
      logMessage.maxLength !== 16384 ||
      logMessage.pattern !== logMessagePattern
    ) {
      fail('LogEntry.message must keep the safe one-line contract');
    }

    const cursor = components.schemas.Cursor;
    if (
      !isRecord(cursor) ||
      cursor.minLength !== 1 ||
      cursor.maxLength !== 4096 ||
      cursor.pattern !== canonicalCursorPattern ||
      !Array.isArray(cursor.examples) ||
      cursor.examples.length !== 1 ||
      cursor.examples[0] !== 'bmV4dA' ||
      !new RegExp(canonicalCursorPattern, 'u').test(cursor.examples[0])
    ) {
      fail('Cursor must use canonical bounded raw URL-base64 form');
    }

    const dataAllowedUnknownHealth = components.schemas.DataAllowedUnknownHealthSnapshot;
    const noDataUnknownHealth = components.schemas.NoDataUnknownHealthSnapshot;
    const unknownHealth = components.schemas.UnknownHealthSnapshot;
    assertExactValues(
      dataAllowedUnknownHealth?.properties?.status?.oneOf?.map((item) => item.$ref),
      [
        '#/components/schemas/AvailableCompleteObservation',
        '#/components/schemas/AvailablePartialObservation',
        '#/components/schemas/StaleObservation',
      ],
      'DataAllowedUnknownHealthSnapshot status union',
    );
    if (dataAllowedUnknownHealth?.properties?.evidence?.maxItems !== 32) {
      fail('data-allowed unknown health must keep bounded evidence');
    }
    if (
      noDataUnknownHealth?.properties?.status?.$ref !==
        '#/components/schemas/UnavailableObservation' ||
      noDataUnknownHealth?.properties?.evidence?.maxItems !== 0
    ) {
      fail('no-data unknown health must forbid evidence');
    }
    assertExactValues(
      unknownHealth?.oneOf?.map((item) => item.$ref),
      [
        '#/components/schemas/DataAllowedUnknownHealthSnapshot',
        '#/components/schemas/NoDataUnknownHealthSnapshot',
      ],
      'UnknownHealthSnapshot union',
    );

    const dataAllowedStatusReferences = [
      '#/components/schemas/AvailableCompleteObservation',
      '#/components/schemas/AvailablePartialObservation',
      '#/components/schemas/StaleObservation',
    ];
    const resultContracts = [
      ['CurrentMetricResult', 100, '#/components/schemas/CurrentMetricSet', false],
      ['EnvironmentPage', 200, '#/components/schemas/Environment', true],
      ['EventPage', 200, '#/components/schemas/InfrastructureEvent', true],
      ['HealthResult', 100, '#/components/schemas/HealthSnapshot', false],
      ['LogBatch', 200, '#/components/schemas/LogEntry', true],
      ['RelationshipPage', 200, '#/components/schemas/Relationship', true],
      ['ResourcePage', 200, '#/components/schemas/ResourceSnapshot', true],
    ];
    for (const [name, maximumItems, itemReference, paged] of resultContracts) {
      const dataAllowedName = `DataAllowed${name}`;
      const noDataName = `NoData${name}`;
      const dataAllowed = components.schemas[dataAllowedName];
      const noData = components.schemas[noDataName];
      const union = components.schemas[name];
      assertExactValues(
        dataAllowed?.properties?.status?.oneOf?.map((item) => item.$ref),
        dataAllowedStatusReferences,
        `${dataAllowedName} status union`,
      );
      if (
        dataAllowed?.properties?.items?.maxItems !== maximumItems ||
        dataAllowed?.properties?.items?.items?.$ref !== itemReference ||
        noData?.properties?.status?.$ref !== '#/components/schemas/UnavailableObservation' ||
        noData?.properties?.items?.maxItems !== 0 ||
        noData?.properties?.items?.items?.$ref !== itemReference
      ) {
        fail(`${name} must bind status availability to its item bound`);
      }
      if (
        paged === true &&
        (dataAllowed?.properties?.page?.$ref !== '#/components/schemas/PageInfo' ||
          noData?.properties?.page?.$ref !== '#/components/schemas/FinalPageInfo')
      ) {
        fail(`${name} must use a final page when data is not allowed`);
      }
      assertExactValues(
        union?.oneOf?.map((item) => item.$ref),
        [`#/components/schemas/${dataAllowedName}`, `#/components/schemas/${noDataName}`],
        `${name} union`,
      );
    }

    const dataAllowedMetricSet = components.schemas.DataAllowedCurrentMetricSet;
    const noDataMetricSet = components.schemas.NoDataCurrentMetricSet;
    const metricSet = components.schemas.CurrentMetricSet;
    assertExactValues(
      dataAllowedMetricSet?.properties?.status?.oneOf?.map((item) => item.$ref),
      dataAllowedStatusReferences,
      'DataAllowedCurrentMetricSet status union',
    );
    if (
      dataAllowedMetricSet?.properties?.samples?.$ref !== '#/components/schemas/MetricSampleSet' ||
      noDataMetricSet?.properties?.status?.$ref !== '#/components/schemas/UnavailableObservation' ||
      noDataMetricSet?.properties?.samples?.$ref !== '#/components/schemas/EmptyMetricSampleSet' ||
      components.schemas.EmptyMetricSampleSet?.additionalProperties !== false ||
      components.schemas.EmptyMetricSampleSet?.maxProperties !== 0
    ) {
      fail('CurrentMetricSet must bind status availability to an empty sealed sample set');
    }
    assertExactValues(
      metricSet?.oneOf?.map((item) => item.$ref),
      [
        '#/components/schemas/DataAllowedCurrentMetricSet',
        '#/components/schemas/NoDataCurrentMetricSet',
      ],
      'CurrentMetricSet union',
    );

    const problem = components.schemas.Problem;
    const expectedProblemProperties = [
      'code',
      'detail',
      'invalid_parameters',
      'request_id',
      'status',
      'title',
      'type',
    ];
    const actualProblemProperties = Object.keys(problem.properties).sort();
    if (
      actualProblemProperties.length !== expectedProblemProperties.length ||
      actualProblemProperties.some((name, index) => name !== expectedProblemProperties[index])
    ) {
      fail('Problem fields must stay on the safe allowlist');
    }

    const responses = components.responses;
    if (!isRecord(responses)) {
      fail('components.responses is required');
    }
    for (const [name, response] of Object.entries(responses)) {
      if (response?.headers?.['Set-Cookie']?.$ref !== '#/components/headers/BrowserStateUpdate') {
        fail(`${name} must document optional authentication cookie updates`);
      }
      if (successResponseNames.has(name)) {
        if (response?.headers?.['X-Request-ID']?.$ref !== '#/components/headers/RequestId') {
          fail(`${name} must return X-Request-ID`);
        }
        if (name === 'MutationAccepted') {
          if ('content' in response) {
            fail('MutationAccepted must have no response body');
          }
          continue;
        }
        if (name === 'ProductEventStream') {
          if (
            response?.content?.['text/event-stream']?.schema?.type !== 'string' ||
            response?.headers?.['Cache-Control']?.schema?.const !== 'no-cache, no-store' ||
            response?.headers?.['X-Accel-Buffering']?.schema?.const !== 'no'
          ) {
            fail('ProductEventStream must keep its bounded SSE transport contract');
          }
          continue;
        }
        const successMedia = response?.content?.['application/json'];
        if (
          !isRecord(successMedia) ||
          successMedia.schema?.$ref !== '#/components/schemas/User' ||
          !isRecord(successMedia.examples) ||
          Object.keys(successMedia.examples).length === 0 ||
          response?.headers?.ETag?.$ref !== '#/components/headers/EntityTag'
        ) {
          fail(`${name} must return one documented versioned user`);
        }
        continue;
      }
      const media = response?.content?.['application/problem+json'];
      if (
        !isRecord(media) ||
        !isRecord(media.examples) ||
        Object.keys(media.examples).length === 0
      ) {
        fail(`${name} must provide a safe problem example`);
      }
      if (response?.headers?.['X-Request-ID']?.$ref !== '#/components/headers/RequestId') {
        fail(`${name} must return X-Request-ID`);
      }
      if (media.schema?.$ref !== '#/components/schemas/Problem') {
        fail(`${name} must return the fixed problem schema`);
      }
      if (
        retryableResponseNames.has(name) &&
        response?.headers?.['Retry-After']?.$ref !== '#/components/headers/RetryAfter'
      ) {
        fail(`${name} must return Retry-After`);
      }
    }

    process.stdout.write(
      `OpenAPI policy passed for ${String(Object.keys(components.schemas).length)} bounded schemas.\n`,
    );
  } finally {
    await rm(temporaryDirectory, { force: true, recursive: true });
  }
};

try {
  await main();
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`${message}\n`);
  process.exitCode = 1;
}
