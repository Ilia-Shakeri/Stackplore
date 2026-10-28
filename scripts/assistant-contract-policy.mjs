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
  'AssistantReadyResponse',
  'AssistantRunResponse',
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

  closed(
    'AssistantRunRequest',
    ['source_id', 'resource_id', 'window_minutes', 'locale', 'question', 'conversation_id'],
    ['source_id', 'resource_id', 'window_minutes', 'locale', 'question'],
  );
  ref('AssistantRunRequest', 'source_id', 'SourceId');
  ref('AssistantRunRequest', 'resource_id', 'ResourceId');
  ref('AssistantRunRequest', 'window_minutes', 'AssistantWindowMinutes');
  ref('AssistantRunRequest', 'locale', 'AssistantLocale');
  ref('AssistantRunRequest', 'conversation_id', 'AssistantConversationId');
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
  check(schemas.AssistantRun?.properties?.goal?.const === 'explain_resource', 'goal.fixed');
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
  closed('AssistantReadyResponse', ['run', 'content_state', 'answer', 'citations']);
  ref('AssistantReadyResponse', 'run', 'AssistantCompletedRun');
  ref('AssistantReadyResponse', 'answer', 'AssistantAnswer');
  const citations = schemas.AssistantReadyResponse?.properties?.citations;
  check(
    schemas.AssistantReadyResponse?.properties?.content_state?.const === 'ready' &&
      citations?.type === 'array' &&
      citations.minItems === 1 &&
      citations.maxItems === 1 &&
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
    schemas.AssistantResourceCitation?.properties?.kind?.const === 'resource',
    'citation.resource',
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
        ...(path === '/assistant/runs' ? ['AssistantIdempotencyKey'] : ['AssistantRunId']),
      ];
      if (path.endsWith('/{citation_id}')) expected.push('AssistantCitationId');
      if (path.endsWith('/events')) expected.push('AssistantLastEventId');
      check(
        same(
          operation?.parameters?.map((item) => item.$ref),
          expected.map(parameterRef),
        ),
        `${policy.operationId}.parameters`,
      );
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
        const target = path.endsWith('/{citation_id}')
          ? 'AssistantResourceCitation'
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
    value.components.schemas.AssistantReadyResponse.properties.citations.maxItems = 64;
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
