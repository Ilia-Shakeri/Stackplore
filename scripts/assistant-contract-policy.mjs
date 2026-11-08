import assert from 'node:assert/strict';

const schemaRef = (name) => `#/components/schemas/${name}`;
const parameterRef = (name) => `#/components/parameters/${name}`;
const same = (actual, expected) =>
  Array.isArray(actual) &&
  actual.length === expected.length &&
  [...actual].sort().every((value, index) => value === [...expected].sort()[index]);

const readResponses = ['200', '400', '401', '403', '404', '413', '429', '500', '503'];
export const assistantRoutePolicies = new Map([
  [
    '/assistant/history',
    {
      get: {
        operationId: 'listAssistantHistory',
        requestBody: false,
        responses: readResponses,
        security: ['BrowserSession'],
      },
    },
  ],
  [
    '/assistant/runs/{run_id}/report',
    {
      get: {
        operationId: 'getAssistantIncidentReport',
        requestBody: false,
        responses: readResponses,
        security: ['BrowserSession'],
      },
    },
  ],
  [
    '/assistant/runs',
    {
      post: {
        operationId: 'createAssistantRun',
        requestBody: true,
        responses: ['202', '400', '401', '403', '404', '409', '413', '415', '429', '500', '503'],
        security: ['BrowserSession', 'CSRFRequestProof'],
      },
    },
  ],
  [
    '/assistant/runs/{run_id}',
    {
      get: {
        operationId: 'getAssistantRun',
        requestBody: false,
        responses: readResponses,
        security: ['BrowserSession'],
      },
    },
  ],
  [
    '/assistant/runs/{run_id}/cancel',
    {
      post: {
        operationId: 'cancelAssistantRun',
        requestBody: false,
        responses: [...readResponses, '409'],
        security: ['BrowserSession', 'CSRFRequestProof'],
      },
    },
  ],
  [
    '/assistant/runs/{run_id}/citations/{citation_id}',
    {
      get: {
        operationId: 'resolveAssistantCitation',
        requestBody: false,
        responses: readResponses,
        security: ['BrowserSession'],
      },
    },
  ],
  [
    '/assistant/runs/{run_id}/events',
    {
      get: {
        operationId: 'streamAssistantRunStatus',
        requestBody: false,
        responses: readResponses,
        security: ['BrowserSession'],
      },
    },
  ],
]);

export const assistantSchemaNames = [
  'AssistantRunId',
  'AssistantConversationId',
  'AssistantCitationId',
  'AssistantEventCursor',
  'AssistantLocale',
  'AssistantWindowMinutes',
  'AssistantRunState',
  'AssistantRunRequest',
  'AssistantTimeRange',
  'AssistantRunScope',
  'AssistantRun',
  'AssistantCompletedRun',
  'AssistantActiveRun',
  'AssistantRunReceipt',
  'AssistantPendingResponse',
  'AssistantLostResponse',
  'AssistantUnavailableResponse',
  'AssistantAnswerText',
  'AssistantFactSection',
  'AssistantOtherSection',
  'AssistantAnswerSection',
  'AssistantAnswer',
  'AssistantResourceCitation',
  'AssistantReadCoverage',
  'AssistantReadyResponse',
  'AssistantRunResponse',
  'AssistantHistoryCursor',
  'AssistantHistoryEntry',
  'AssistantHistoryResponse',
  'AssistantIncidentMetric',
  'AssistantIncidentObservation',
  'AssistantIncidentHypothesis',
  'AssistantIncidentReport',
  'AssistantIncidentReportReadyResponse',
  'AssistantIncidentReportResponse',
  'AssistantStatusEvent',
  'AssistantResetEvent',
];
export const assistantParameterNames = [
  'AssistantRunId',
  'AssistantCitationId',
  'AssistantIdempotencyKey',
  'AssistantLastEventId',
];

// Keep the cursor as a string so decimal positions survive browser number limits.
export const assistantCursorPattern = (() => {
  const maximum = '9223372036854775807';
  const alternatives = ['0', '[1-9][0-9]{0,17}'];
  for (let index = 0; index < maximum.length; index += 1) {
    const lower = index === 0 ? 1 : 0;
    const upper = Number(maximum[index]) - 1;
    if (upper < lower) continue;
    const digit = upper === lower ? String(lower) : `[${String(lower)}-${String(upper)}]`;
    const remaining = maximum.length - index - 1;
    alternatives.push(
      `${maximum.slice(0, index)}${digit}${remaining === 0 ? '' : `[0-9]{${String(remaining)}}`}`,
    );
  }
  alternatives.push(maximum);
  return `^(?:${alternatives.join('|')})$(?![\\s\\S])`;
})();

export const inspectAssistantPolicy = (document) => {
  const issues = [];
  const schemas = document.components?.schemas ?? {};
  const parameters = document.components?.parameters ?? {};
  const check = (condition, code) => {
    if (!condition) issues.push(`assistant.${code}`);
  };
  const closed = (name, fields, required = fields) => {
    const schema = schemas[name];
    check(
      schema?.type === 'object' &&
        schema.additionalProperties === false &&
        same(Object.keys(schema.properties ?? {}), fields) &&
        same(schema.required, required),
      `${name}.closed`,
    );
  };
  const ref = (name, field, target) =>
    check(schemas[name]?.properties?.[field]?.$ref === schemaRef(target), `${name}.${field}`);
  const bounded = (value, minimum, maximum) =>
    value?.type === 'integer' && value.minimum === minimum && value.maximum === maximum;

  closed('AssistantHistoryEntry', [
    'id',
    'source_id',
    'resource_id',
    'goal',
    'state',
    'created_at',
    'updated_at',
    'range',
    'tool_calls',
    'inference_calls',
    'usage_uncertain',
  ]);
  closed('AssistantHistoryResponse', ['entries', 'next_cursor']);
  for (const [field, target] of [
    ['id', 'AssistantRunId'],
    ['source_id', 'SourceId'],
    ['state', 'AssistantRunState'],
    ['created_at', 'Timestamp'],
    ['updated_at', 'Timestamp'],
    ['range', 'AssistantTimeRange'],
  ])
    ref('AssistantHistoryEntry', field, target);
  const historyEntry = schemas.AssistantHistoryEntry?.properties;
  const historyPage = schemas.AssistantHistoryResponse?.properties;
  check(
    schemas.AssistantHistoryCursor?.type === 'string' &&
      schemas.AssistantHistoryCursor.minLength === 45 &&
      schemas.AssistantHistoryCursor.maxLength === 768 &&
      schemas.AssistantHistoryCursor.pattern === '^[A-Za-z0-9_-]{1,724}\\.[A-Za-z0-9_-]{43}$' &&
      same(historyEntry?.goal?.enum, ['explain_resource', 'investigate_incident']) &&
      historyEntry?.resource_id?.oneOf?.length === 2 &&
      historyEntry.resource_id.oneOf[0].$ref === schemaRef('ResourceId') &&
      historyEntry.resource_id.oneOf[1].type === 'null' &&
      bounded(historyEntry?.tool_calls, 0, 12) &&
      bounded(historyEntry?.inference_calls, 0, 3) &&
      historyEntry?.usage_uncertain?.type === 'boolean' &&
      historyPage?.entries?.type === 'array' &&
      historyPage.entries.maxItems === 10 &&
      historyPage.entries.items?.$ref === schemaRef('AssistantHistoryEntry') &&
      historyPage?.next_cursor?.oneOf?.length === 2 &&
      historyPage.next_cursor.oneOf[0].$ref === schemaRef('AssistantHistoryCursor') &&
      historyPage.next_cursor.oneOf[1].type === 'null',
    'history.closed_bounds',
  );

  closed(
    'AssistantRunRequest',
    [
      'source_id',
      'resource_id',
      'window_minutes',
      'locale',
      'question',
      'conversation_id',
      'goal',
      'log_consent',
    ],
    ['source_id', 'resource_id', 'window_minutes', 'locale', 'question'],
  );
  ref('AssistantRunRequest', 'source_id', 'SourceId');
  ref('AssistantRunRequest', 'resource_id', 'ResourceId');
  ref('AssistantRunRequest', 'window_minutes', 'AssistantWindowMinutes');
  ref('AssistantRunRequest', 'locale', 'AssistantLocale');
  ref('AssistantRunRequest', 'conversation_id', 'AssistantConversationId');
  check(
    same(schemas.AssistantRunRequest?.properties?.goal?.enum, [
      'explain_resource',
      'investigate_incident',
    ]) && schemas.AssistantRunRequest.properties.goal.default === 'explain_resource',
    'request.goal',
  );
  check(
    schemas.AssistantRunRequest?.properties?.log_consent?.type === 'boolean' &&
      schemas.AssistantRunRequest.properties.log_consent.default === false,
    'request.log_consent',
  );
  const question = schemas.AssistantRunRequest?.properties?.question;
  check(
    question?.type === 'string' &&
      question.minLength === 1 &&
      question.maxLength === 8192 &&
      question.pattern === '^(?!\\p{White_Space}*$)[^\\u0000]+$' &&
      question.description?.includes('8192 UTF-8 bytes'),
    'question.byte_bound',
  );
  check(same(schemas.AssistantLocale?.enum, ['en', 'fa']), 'locale.closed');
  check(
    same(schemas.AssistantWindowMinutes?.enum, [5, 15, 60, 360, 1440]) &&
      bounded(schemas.AssistantWindowMinutes, 5, 1440),
    'window.closed',
  );
  check(
    same(schemas.AssistantRunState?.enum, [
      'queued',
      'running',
      'completed',
      'failed',
      'cancelled',
      'expired',
      'interrupted',
      'uncertain',
    ]),
    'states.closed',
  );
  for (const [name, prefix] of [
    ['AssistantRunId', 'ar'],
    ['AssistantConversationId', 'ac'],
    ['AssistantCitationId', 'ev'],
  ]) {
    check(
      schemas[name]?.type === 'string' &&
        schemas[name].minLength === 67 &&
        schemas[name].maxLength === 67 &&
        schemas[name].pattern === `^${prefix}_[0-9a-f]{64}$`,
      `${name}.canonical`,
    );
  }
  check(
    schemas.AssistantEventCursor?.type === 'string' &&
      schemas.AssistantEventCursor.minLength === 1 &&
      schemas.AssistantEventCursor.maxLength === 19 &&
      schemas.AssistantEventCursor.pattern === assistantCursorPattern,
    'cursor.int64',
  );
  closed('AssistantTimeRange', ['from', 'to']);
  ref('AssistantTimeRange', 'from', 'Timestamp');
  ref('AssistantTimeRange', 'to', 'Timestamp');
  closed('AssistantRunScope', ['source_id', 'resource_id', 'window_minutes', 'range']);
  ref('AssistantRunScope', 'source_id', 'SourceId');
  ref('AssistantRunScope', 'resource_id', 'ResourceId');
  ref('AssistantRunScope', 'window_minutes', 'AssistantWindowMinutes');
  ref('AssistantRunScope', 'range', 'AssistantTimeRange');
  closed('AssistantRun', [
    'id',
    'conversation_id',
    'goal',
    'state',
    'version',
    'scope',
    'created_at',
    'updated_at',
    'deadline',
    'usage_uncertain',
    'usage_conservative',
  ]);
  check(
    same(Object.keys(schemas.AssistantRun?.properties?.goal ?? {}), ['type', 'enum']) &&
      same(schemas.AssistantRun?.properties?.goal?.enum, [
        'explain_resource',
        'investigate_incident',
      ]),
    'goal.fixed',
  );
  check(bounded(schemas.AssistantRun?.properties?.version, 1, 64), 'version.bounded');
  ref('AssistantRun', 'id', 'AssistantRunId');
  ref('AssistantRun', 'conversation_id', 'AssistantConversationId');
  ref('AssistantRun', 'state', 'AssistantRunState');
  ref('AssistantRun', 'scope', 'AssistantRunScope');
  for (const [name, states] of [
    ['AssistantActiveRun', ['queued', 'running']],
    ['AssistantCompletedRun', ['completed']],
  ]) {
    const parts = schemas[name]?.allOf;
    const state = parts?.[1]?.properties?.state;
    check(
      Array.isArray(parts) &&
        parts.length === 2 &&
        parts[0]?.$ref === schemaRef('AssistantRun') &&
        same(Object.keys(parts[1] ?? {}), ['properties', 'required']) &&
        same(parts[1]?.required, ['state']) &&
        same(Object.keys(parts[1]?.properties ?? {}), ['state']) &&
        state?.type === 'string' &&
        (states.length === 1 ? state.const === states[0] : same(state.enum, states)),
      `${name}.state`,
    );
  }
  closed('AssistantRunReceipt', ['run']);
  ref('AssistantRunReceipt', 'run', 'AssistantRun');
  for (const [name, contentState, runType] of [
    ['AssistantPendingResponse', 'pending', 'AssistantActiveRun'],
    ['AssistantLostResponse', 'volatile_lost', 'AssistantCompletedRun'],
    ['AssistantUnavailableResponse', 'unavailable', 'AssistantRun'],
  ]) {
    closed(name, ['run', 'content_state']);
    ref(name, 'run', runType);
    check(schemas[name]?.properties?.content_state?.const === contentState, `${name}.state`);
  }
  closed(
    'AssistantReadyResponse',
    ['run', 'content_state', 'answer', 'citations', 'coverage'],
    ['run', 'content_state', 'answer', 'citations'],
  );
  ref('AssistantReadyResponse', 'run', 'AssistantCompletedRun');
  ref('AssistantReadyResponse', 'answer', 'AssistantAnswer');
  const citations = schemas.AssistantReadyResponse?.properties?.citations;
  check(
    schemas.AssistantReadyResponse?.properties?.content_state?.const === 'ready' &&
      citations?.type === 'array' &&
      citations.minItems === 1 &&
      citations.maxItems === 64 &&
      citations.items?.$ref === schemaRef('AssistantResourceCitation'),
    'ready.exact_resource',
  );
  check(
    same(
      schemas.AssistantRunResponse?.oneOf?.map((item) => item.$ref),
      [
        'AssistantPendingResponse',
        'AssistantLostResponse',
        'AssistantUnavailableResponse',
        'AssistantReadyResponse',
      ].map(schemaRef),
    ),
    'response.branches',
  );
  closed('AssistantResourceCitation', [
    'id',
    'kind',
    'source_id',
    'resource_id',
    'observed_at',
    'range',
  ]);
  check(
    same(schemas.AssistantResourceCitation?.properties?.kind?.enum, [
      'resource',
      'health',
      'metric',
      'event',
      'relationship',
      'log',
      'alert',
      'workload',
    ]),
    'citation.resource',
  );
  closed('AssistantReadCoverage', [
    'tool',
    'availability',
    'completeness',
    'records',
    'more_available',
  ]);
  check(
    same(schemas.AssistantReadCoverage?.properties?.tool?.enum, [
      'inventory',
      'health',
      'metrics',
      'events',
      'topology',
      'logs',
      'alert_history',
      'workload_history',
    ]),
    'coverage.tools',
  );
  check(
    same(schemas.AssistantReadCoverage?.properties?.availability?.enum, [
      'available',
      'stale',
      'unavailable',
      'unsupported',
    ]),
    'coverage.availability',
  );
  check(
    same(schemas.AssistantReadCoverage?.properties?.completeness?.enum, ['complete', 'partial']),
    'coverage.completeness',
  );
  check(
    bounded(schemas.AssistantReadCoverage?.properties?.records, 0, 8) &&
      schemas.AssistantReadCoverage?.properties?.more_available?.type === 'boolean',
    'coverage.fields',
  );
  const coverage = schemas.AssistantReadyResponse?.properties?.coverage;
  check(
    coverage?.type === 'array' &&
      coverage.minItems === 7 &&
      coverage.maxItems === 8 &&
      coverage.items?.$ref === schemaRef('AssistantReadCoverage'),
    'coverage.bounded',
  );
  for (const [field, target] of [
    ['id', 'AssistantCitationId'],
    ['source_id', 'SourceId'],
    ['resource_id', 'ResourceId'],
    ['observed_at', 'Timestamp'],
    ['range', 'AssistantTimeRange'],
  ])
    ref('AssistantResourceCitation', field, target);
  closed('AssistantAnswer', ['sections']);
  const sections = schemas.AssistantAnswer?.properties?.sections;
  check(
    sections?.type === 'array' &&
      sections.minItems === 1 &&
      sections.maxItems === 12 &&
      sections.items?.$ref === schemaRef('AssistantAnswerSection'),
    'sections.bounded',
  );
  check(
    same(
      schemas.AssistantAnswerSection?.oneOf?.map((item) => item.$ref),
      [schemaRef('AssistantFactSection'), schemaRef('AssistantOtherSection')],
    ),
    'sections.closed',
  );
  for (const name of ['AssistantFactSection', 'AssistantOtherSection']) {
    closed(name, ['kind', 'text', 'citations']);
    ref(name, 'text', 'AssistantAnswerText');
    const refs = schemas[name]?.properties?.citations;
    check(
      refs?.type === 'array' &&
        refs.maxItems === 64 &&
        refs.uniqueItems === true &&
        refs.items?.$ref === schemaRef('AssistantCitationId') &&
        (name === 'AssistantFactSection' ? refs.minItems === 1 : (refs.minItems ?? 0) === 0),
      `${name}.citations`,
    );
  }
  check(
    schemas.AssistantFactSection?.properties?.kind?.const === 'facts' &&
      same(schemas.AssistantOtherSection?.properties?.kind?.enum, [
        'hypotheses',
        'gaps',
        'next_checks',
      ]),
    'sections.kinds',
  );
  check(
    schemas.AssistantAnswerText?.type === 'string' &&
      schemas.AssistantAnswerText.minLength === 1 &&
      schemas.AssistantAnswerText.maxLength === 4096 &&
      schemas.AssistantAnswerText.pattern ===
        '^(?!\\p{White_Space}*$)(?![\\s\\S]*(?:\\]\\(|http://|https://))[^<>\\u0000]+$' &&
      schemas.AssistantAnswerText.description?.includes('4096 UTF-8 bytes'),
    'answer.inert_byte_bound',
  );
  closed('AssistantIncidentMetric', ['name', 'unit', 'value', 'interval_seconds']);
  check(
    same(schemas.AssistantIncidentMetric?.properties?.name?.enum, [
      'cpu.usage.cores',
      'memory.usage.bytes',
      'restart.count',
      'network.receive.bytes_total',
      'network.send.bytes_total',
      'cpu.request.cores',
      'cpu.limit.cores',
      'memory.request.bytes',
      'memory.limit.bytes',
    ]) &&
      same(schemas.AssistantIncidentMetric?.properties?.unit?.enum, ['cores', 'bytes', 'count']) &&
      schemas.AssistantIncidentMetric?.properties?.value?.type === 'number' &&
      schemas.AssistantIncidentMetric.properties.value.minimum === 0 &&
      schemas.AssistantIncidentMetric.properties.value.maximum === 9007199254740991 &&
      bounded(schemas.AssistantIncidentMetric?.properties?.interval_seconds, 0, 86400),
    'report.metric',
  );
  const observationFields = [
    'citation_id',
    'kind',
    'at',
    'time_basis',
    'availability',
    'completeness',
    'synthetic',
    'health',
    'metric',
    'severity',
    'redacted',
    'truncated',
  ];
  closed(
    'AssistantIncidentObservation',
    [...observationFields, 'alert', 'workload'],
    observationFields,
  );
  ref('AssistantIncidentObservation', 'citation_id', 'AssistantCitationId');
  ref('AssistantIncidentObservation', 'at', 'Timestamp');
  ref('AssistantIncidentObservation', 'completeness', 'Completeness');
  const observation = schemas.AssistantIncidentObservation?.properties;
  const alert = observation?.alert;
  const workload = observation?.workload;
  check(
    alert?.type === 'object' &&
      alert.additionalProperties === false &&
      same(alert.required, ['from', 'to']) &&
      same(Object.keys(alert.properties ?? {}), ['from', 'to']) &&
      alert.properties.from.oneOf?.length === 2 &&
      alert.properties.from.oneOf[0].type === 'string' &&
      same(alert.properties.from.oneOf[0].enum, [
        'pending',
        'firing',
        'acknowledged',
        'resolved',
      ]) &&
      alert.properties.from.oneOf[1].type === 'null' &&
      alert.properties.to.type === 'string' &&
      same(alert.properties.to.enum, ['pending', 'firing', 'acknowledged', 'resolved']) &&
      workload?.type === 'object' &&
      workload.additionalProperties === false &&
      same(workload.required, ['desired', 'ready']) &&
      same(Object.keys(workload.properties ?? {}), ['desired', 'ready']) &&
      ['desired', 'ready'].every(
        (key) =>
          workload.properties[key]?.oneOf?.length === 2 &&
          bounded(workload.properties[key].oneOf[0], 0, 9007199254740991) &&
          workload.properties[key].oneOf[1].type === 'null',
      ),
    'report.history',
  );
  check(
    same(observation?.kind?.enum, [
      'resource',
      'health',
      'metric',
      'event',
      'relationship',
      'log',
      'alert',
      'workload',
    ]) &&
      same(observation?.time_basis?.enum, ['observed', 'sampled', 'occurred']) &&
      same(observation?.availability?.enum, ['available', 'stale', 'unavailable', 'unsupported']) &&
      same(observation?.health?.enum, ['', 'healthy', 'degraded', 'unhealthy', 'unknown']) &&
      same(observation?.severity?.enum, [
        '',
        'trace',
        'debug',
        'info',
        'warning',
        'error',
        'critical',
        'unknown',
      ]) &&
      ['synthetic', 'redacted', 'truncated'].every(
        (key) => observation?.[key]?.type === 'boolean',
      ) &&
      observation?.metric?.oneOf?.length === 2 &&
      observation.metric.oneOf[0].$ref === schemaRef('AssistantIncidentMetric') &&
      observation.metric.oneOf[1].type === 'null',
    'report.observation',
  );
  closed('AssistantIncidentHypothesis', ['code', 'rank', 'qualified', 'supporting', 'contrary']);
  const hypothesis = schemas.AssistantIncidentHypothesis?.properties;
  check(
    same(hypothesis?.code?.enum, [
      'memory_pressure_possible',
      'cpu_pressure_possible',
      'restart_related_possible',
      'health_problem_possible',
      'workload_shortfall_possible',
    ]) &&
      bounded(hypothesis?.rank, 1, 5) &&
      hypothesis?.qualified?.type === 'boolean' &&
      hypothesis.qualified.const === true &&
      ['supporting', 'contrary'].every(
        (key) =>
          hypothesis?.[key]?.type === 'array' &&
          hypothesis[key].maxItems === 2 &&
          hypothesis[key].uniqueItems === true &&
          hypothesis[key].items?.$ref === schemaRef('AssistantCitationId'),
      ) &&
      hypothesis.supporting.minItems === 1 &&
      (hypothesis.contrary.minItems ?? 0) === 0,
    'report.hypothesis',
  );
  closed('AssistantIncidentReport', ['timeline', 'hypotheses', 'gaps', 'next_reads']);
  const report = schemas.AssistantIncidentReport?.properties;
  check(
    report?.timeline?.type === 'array' &&
      report.timeline.minItems === 1 &&
      report.timeline.maxItems === 64 &&
      report.timeline.items?.$ref === schemaRef('AssistantIncidentObservation') &&
      report?.hypotheses?.type === 'array' &&
      report.hypotheses.maxItems === 5 &&
      report.hypotheses.items?.$ref === schemaRef('AssistantIncidentHypothesis') &&
      report?.gaps?.type === 'array' &&
      report.gaps.minItems === 1 &&
      report.gaps.maxItems === 12 &&
      report.gaps.uniqueItems === true &&
      same(report.gaps.items?.enum, [
        'alerts_not_read',
        'workload_history_not_read',
        'cause_unproven',
        'limited_evidence',
        'matching_limits_missing',
        'metrics_missing',
        'events_missing',
        'conflicting_samples',
        'restart_counter_reset',
        'conflicting_health',
        'out_of_window_observation',
      ]) &&
      report?.next_reads?.type === 'array' &&
      report.next_reads.minItems === 5 &&
      report.next_reads.maxItems === 5 &&
      report.next_reads.uniqueItems === true &&
      same(report.next_reads.items?.enum, [
        'review_health',
        'review_metrics',
        'review_events',
        'review_alert_history',
        'review_workload_history',
      ]),
    'report.bounds',
  );
  closed('AssistantIncidentReportReadyResponse', [
    'run',
    'content_state',
    'report',
    'citations',
    'coverage',
  ]);
  ref('AssistantIncidentReportReadyResponse', 'run', 'AssistantCompletedRun');
  ref('AssistantIncidentReportReadyResponse', 'report', 'AssistantIncidentReport');
  const reportReady = schemas.AssistantIncidentReportReadyResponse?.properties;
  check(
    reportReady?.content_state?.const === 'ready' &&
      reportReady?.citations?.type === 'array' &&
      reportReady.citations.minItems === 1 &&
      reportReady.citations.maxItems === 64 &&
      reportReady.citations.items?.$ref === schemaRef('AssistantResourceCitation') &&
      reportReady?.coverage?.type === 'array' &&
      reportReady.coverage.minItems === 7 &&
      reportReady.coverage.maxItems === 8 &&
      reportReady.coverage.items?.$ref === schemaRef('AssistantReadCoverage') &&
      same(
        schemas.AssistantIncidentReportResponse?.oneOf?.map((item) => item.$ref),
        [
          'AssistantPendingResponse',
          'AssistantLostResponse',
          'AssistantUnavailableResponse',
          'AssistantIncidentReportReadyResponse',
        ].map(schemaRef),
      ),
    'report.response',
  );
  closed('AssistantStatusEvent', ['run_id', 'sequence', 'version', 'state', 'at']);
  ref('AssistantStatusEvent', 'run_id', 'AssistantRunId');
  ref('AssistantStatusEvent', 'state', 'AssistantRunState');
  ref('AssistantStatusEvent', 'at', 'Timestamp');
  check(
    bounded(schemas.AssistantStatusEvent?.properties?.sequence, 1, 64) &&
      bounded(schemas.AssistantStatusEvent?.properties?.version, 1, 64),
    'stream.status_bounds',
  );
  closed('AssistantResetEvent', ['run_id', 'reason']);
  ref('AssistantResetEvent', 'run_id', 'AssistantRunId');
  check(
    same(schemas.AssistantResetEvent?.properties?.reason?.enum, [
      'replay_gap',
      'cursor_ahead',
      'replay_expired',
      'queue_overflow',
    ]),
    'stream.reset_reasons',
  );
  for (const [name, wireName, location, required, target] of [
    ['AssistantRunId', 'run_id', 'path', true, 'AssistantRunId'],
    ['AssistantCitationId', 'citation_id', 'path', true, 'AssistantCitationId'],
    ['AssistantLastEventId', 'Last-Event-ID', 'header', false, 'AssistantEventCursor'],
  ]) {
    const parameter = parameters[name];
    check(
      parameter?.name === wireName &&
        parameter.in === location &&
        parameter.required === required &&
        parameter.schema?.$ref === schemaRef(target),
      `${name}.parameter`,
    );
  }
  const key = parameters.AssistantIdempotencyKey;
  check(
    key?.name === 'Idempotency-Key' &&
      key.in === 'header' &&
      key.required === true &&
      key.schema?.type === 'string' &&
      key.schema.minLength === 64 &&
      key.schema.maxLength === 64 &&
      key.schema.pattern === '^[0-9a-f]{64}$',
    'idempotency.header',
  );
  for (const [path, methods] of assistantRoutePolicies) {
    for (const [method, policy] of Object.entries(methods)) {
      const operation = document.paths?.[path]?.[method];
      const expected = [
        'RequestId',
        ...(path === '/assistant/history'
          ? []
          : path === '/assistant/runs'
            ? ['AssistantIdempotencyKey']
            : ['AssistantRunId']),
      ];
      if (path.endsWith('/{citation_id}')) expected.push('AssistantCitationId');
      if (path.endsWith('/events')) expected.push('AssistantLastEventId');
      check(
        same(
          operation?.parameters
            ?.filter((item) => path !== '/assistant/history' || item.$ref !== undefined)
            .map((item) => item.$ref),
          expected.map(parameterRef),
        ),
        `${policy.operationId}.parameters`,
      );
      if (path === '/assistant/history') {
        const query = operation?.parameters?.filter((item) => item.in === 'query') ?? [];
        check(
          operation?.parameters?.length === 4 &&
            same(
              query.map((item) => item.name),
              ['state', 'limit', 'cursor'],
            ) &&
            query.every(
              (item) =>
                item.required !== true &&
                (item.name === 'state'
                  ? item.schema?.$ref === schemaRef('AssistantRunState')
                  : item.name === 'cursor'
                    ? item.schema?.$ref === schemaRef('AssistantHistoryCursor')
                    : bounded(item.schema, 1, 10) && item.schema.default === 10),
            ),
          'history.parameters',
        );
      }
      check(
        Array.isArray(operation?.security) &&
          operation.security.length === 1 &&
          same(Object.keys(operation.security[0]), policy.security) &&
          Object.values(operation.security[0]).every(
            (value) => Array.isArray(value) && value.length === 0,
          ),
        `${policy.operationId}.proofs`,
      );
      if (policy.requestBody) {
        check(
          operation?.requestBody?.required === true &&
            operation.requestBody.content?.['application/json']?.schema?.$ref ===
              schemaRef('AssistantRunRequest') &&
            operation.description?.includes('64 KiB') &&
            operation.description?.includes('8192'),
          'request.body',
        );
      } else check(!('requestBody' in (operation ?? {})), `${policy.operationId}.no_body`);
      const status = path === '/assistant/runs' ? '202' : '200';
      const response = operation?.responses?.[status];
      if (path.endsWith('/events')) {
        check(response?.$ref === '#/components/responses/AssistantStatusStream', 'stream.response');
      } else {
        const target =
          path === '/assistant/history'
            ? 'AssistantHistoryResponse'
            : path.endsWith('/{citation_id}')
              ? 'AssistantResourceCitation'
              : path.endsWith('/report')
                ? 'AssistantIncidentReportResponse'
                : method === 'get'
                  ? 'AssistantRunResponse'
                  : 'AssistantRunReceipt';
        check(
          response?.headers?.['Cache-Control']?.schema?.const === 'no-store' &&
            response.headers?.['Set-Cookie']?.$ref === '#/components/headers/BrowserStateUpdate' &&
            response.headers?.['X-Request-ID']?.$ref === '#/components/headers/RequestId' &&
            same(Object.keys(response.content ?? {}), ['application/json']) &&
            response.content['application/json']?.schema?.$ref === schemaRef(target),
          `${policy.operationId}.private_response`,
        );
      }
    }
  }
  const stream = document.components?.responses?.AssistantStatusStream;
  check(
    stream?.headers?.['Cache-Control']?.schema?.const === 'no-store' &&
      stream.headers?.['X-Accel-Buffering']?.schema?.const === 'no' &&
      stream.content?.['text/event-stream']?.schema?.type === 'string' &&
      stream.description?.includes('16 KiB') &&
      stream.description?.includes('100 statuses'),
    'stream.private_bounded',
  );
  return issues;
};

// These mutations exercise security and state correlations, not descriptive formatting.
export const runAssistantPolicySelfTests = (document) => {
  assert.deepEqual(inspectAssistantPolicy(document), []);
  const focused = {
    paths: Object.fromEntries(
      [...assistantRoutePolicies.keys()].map((path) => [path, document.paths[path]]),
    ),
    components: {
      schemas: Object.fromEntries(
        assistantSchemaNames.map((name) => [name, document.components.schemas[name]]),
      ),
      parameters: Object.fromEntries(
        assistantParameterNames.map((name) => [name, document.components.parameters[name]]),
      ),
      responses: { AssistantStatusStream: document.components.responses.AssistantStatusStream },
    },
  };
  const reject = (code, mutate) => {
    const candidate = globalThis.structuredClone(focused);
    mutate(candidate);
    assert.ok(inspectAssistantPolicy(candidate).includes(`assistant.${code}`), code);
  };
  reject('AssistantRunRequest.closed', (value) => {
    value.components.schemas.AssistantRunRequest.additionalProperties = true;
  });
  reject('AssistantHistoryEntry.closed', (value) => {
    value.components.schemas.AssistantHistoryEntry.properties.question = { type: 'string' };
  });
  reject('history.closed_bounds', (value) => {
    value.components.schemas.AssistantHistoryResponse.properties.entries.maxItems = 11;
  });
  reject('history.parameters', (value) => {
    value.paths['/assistant/history'].get.parameters.push({
      name: 'owner',
      in: 'query',
      schema: { type: 'string' },
    });
  });
  reject('report.hypothesis', (value) => {
    value.components.schemas.AssistantIncidentHypothesis.properties.qualified.const = false;
  });
  reject('report.hypothesis', (value) => {
    value.components.schemas.AssistantIncidentHypothesis.properties.code.enum.push('cause_proven');
  });
  reject('AssistantIncidentObservation.closed', (value) => {
    value.components.schemas.AssistantIncidentObservation.properties.message = { type: 'string' };
  });
  reject('report.bounds', (value) => {
    value.components.schemas.AssistantIncidentReport.properties.timeline.maxItems = 65;
  });
  reject('getAssistantIncidentReport.no_body', (value) => {
    value.paths['/assistant/runs/{run_id}/report'].get.requestBody = { required: true };
  });
  reject('AssistantRunRequest.closed', (value) => {
    value.components.schemas.AssistantRunRequest.properties.profile = {
      type: 'string',
      maxLength: 128,
    };
  });
  reject('question.byte_bound', (value) => {
    value.components.schemas.AssistantRunRequest.properties.question.maxLength = 8193;
  });
  reject('window.closed', (value) => {
    value.components.schemas.AssistantWindowMinutes.enum.push(30);
  });
  reject('goal.fixed', (value) => {
    value.components.schemas.AssistantRun.properties.goal.const = 'investigate_incident';
  });
  reject('goal.fixed', (value) => {
    value.components.schemas.AssistantRun.properties.goal.enum.push('shell');
  });
  reject('AssistantRun.closed', (value) => {
    value.components.schemas.AssistantRun.properties.question = { type: 'string', maxLength: 8192 };
  });
  reject('AssistantPendingResponse.run', (value) => {
    value.components.schemas.AssistantPendingResponse.properties.run.$ref =
      schemaRef('AssistantRun');
  });
  reject('AssistantLostResponse.run', (value) => {
    value.components.schemas.AssistantLostResponse.properties.run.$ref = schemaRef('AssistantRun');
  });
  reject('AssistantUnavailableResponse.closed', (value) => {
    value.components.schemas.AssistantUnavailableResponse.properties.answer = {
      $ref: schemaRef('AssistantAnswer'),
    };
  });
  reject('AssistantCompletedRun.state', (value) => {
    value.components.schemas.AssistantCompletedRun.allOf[1].properties.state.const = 'running';
  });
  reject('ready.exact_resource', (value) => {
    value.components.schemas.AssistantReadyResponse.properties.citations.maxItems = 65;
  });
  reject('request.goal', (value) => {
    value.components.schemas.AssistantRunRequest.properties.goal.enum.push('shell');
  });
  reject('request.log_consent', (value) => {
    value.components.schemas.AssistantRunRequest.properties.log_consent.default = true;
  });
  reject('coverage.tools', (value) => {
    value.components.schemas.AssistantReadCoverage.properties.tool.enum.push('sql');
  });
  reject('coverage.availability', (value) => {
    value.components.schemas.AssistantReadCoverage.properties.availability.enum.push('healthy');
  });
  reject('coverage.fields', (value) => {
    value.components.schemas.AssistantReadCoverage.properties.records.maximum = 9;
  });
  reject('coverage.bounded', (value) => {
    value.components.schemas.AssistantReadyResponse.properties.coverage.maxItems = 7;
  });
  reject('AssistantResourceCitation.closed', (value) => {
    value.components.schemas.AssistantResourceCitation.properties.href = {
      type: 'string',
      maxLength: 1024,
    };
  });
  reject('AssistantFactSection.citations', (value) => {
    value.components.schemas.AssistantFactSection.properties.citations.minItems = 0;
  });
  reject('AssistantOtherSection.citations', (value) => {
    value.components.schemas.AssistantOtherSection.properties.citations.uniqueItems = false;
  });
  reject('sections.kinds', (value) => {
    value.components.schemas.AssistantOtherSection.properties.kind.enum.push('command');
  });
  reject('sections.bounded', (value) => {
    value.components.schemas.AssistantAnswer.properties.sections.maxItems = 13;
  });
  reject('answer.inert_byte_bound', (value) => {
    delete value.components.schemas.AssistantAnswerText.pattern;
  });
  reject('AssistantStatusEvent.closed', (value) => {
    value.components.schemas.AssistantStatusEvent.properties.answer = {
      $ref: schemaRef('AssistantAnswer'),
    };
  });
  reject('stream.status_bounds', (value) => {
    value.components.schemas.AssistantStatusEvent.properties.sequence.maximum = 65;
  });
  reject('AssistantResetEvent.closed', (value) => {
    value.components.schemas.AssistantResetEvent.properties.citations = {
      type: 'array',
      maxItems: 1,
    };
  });
  reject('idempotency.header', (value) => {
    value.components.parameters.AssistantIdempotencyKey.in = 'query';
  });
  reject('cursor.int64', (value) => {
    value.components.schemas.AssistantEventCursor.pattern = '^[0-9]{1,19}$';
  });
  reject('createAssistantRun.proofs', (value) => {
    value.paths['/assistant/runs'].post.security = [{ BrowserSession: [] }];
  });
  reject('getAssistantRun.proofs', (value) => {
    value.paths['/assistant/runs/{run_id}'].get.security.push({});
  });
  reject('cancelAssistantRun.no_body', (value) => {
    value.paths['/assistant/runs/{run_id}/cancel'].post.requestBody = {};
  });
  reject('getAssistantRun.private_response', (value) => {
    value.paths['/assistant/runs/{run_id}'].get.responses['200'].headers[
      'Cache-Control'
    ].schema.const = 'public';
  });
  reject('stream.private_bounded', (value) => {
    value.components.responses.AssistantStatusStream.headers['Cache-Control'].schema.const =
      'public';
  });

  const cursor = new RegExp(assistantCursorPattern, 'u');
  for (const value of ['0', '1', '64', '65', '9007199254740993', '9223372036854775807'])
    assert.ok(cursor.test(value));
  for (const value of [
    '',
    '-1',
    '+1',
    '00',
    '01',
    '1.0',
    '1e2',
    ' 1',
    '1\n',
    '۱',
    '１',
    '9223372036854775808',
    '9999999999999999999',
  ])
    assert.ok(!cursor.test(value), value);
  const maximum = 9223372036854775807n;
  for (let exponent = 0n; exponent <= 19n; exponent += 1n) {
    const power = 10n ** exponent;
    for (const value of [power - 1n, power, power + 1n, maximum - power, maximum + power]) {
      assert.equal(cursor.test(String(value)), value >= 0n && value <= maximum, String(value));
    }
  }
  const question = new RegExp(
    document.components.schemas.AssistantRunRequest.properties.question.pattern,
    'u',
  );
  const text = new RegExp(document.components.schemas.AssistantAnswerText.pattern, 'u');
  for (const value of ['', ' ', '\n\t', '\u0085', '\u2003', 'x\u0000'])
    assert.ok(!question.test(value), 'question text');
  for (const value of ['Inspect resource.', 'این منبع را بررسی کن.', '\ufeff'])
    assert.ok(question.test(value), 'valid question text');
  for (const value of [
    '',
    ' ',
    '<tag>',
    '[text](path)',
    'http://local',
    'https://local',
    'x\u0000',
  ])
    assert.ok(!text.test(value), 'inert answer text');
  assert.ok(text.test('Observed resource; safe next read.'));
};
