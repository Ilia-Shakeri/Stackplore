import assert from 'node:assert/strict';

export const runbookSchemaNames = [
  'RunbookId',
  'RunbookDigest',
  'RunbookDocument',
  'RunbookCorpus',
  'RunbookMaterial',
  'RunbookPreview',
  'RunbookCommit',
  'RunbookRevision',
  'RunbookCatalog',
  'RunbookReference',
  'RunbookSearch',
  'RunbookHit',
  'RunbookResults',
];
const reads = ['200', '400', '401', '403', '413', '429', '500', '503'];
const writes = [...reads, '409', '415'];
export const runbookRoutePolicies = new Map([
  [
    '/assistant/runbooks',
    {
      get: {
        operationId: 'listRunbookCatalog',
        requestBody: false,
        responses: reads,
        security: ['BrowserSession'],
      },
    },
  ],
  ...[
    ['preview', 'previewRunbookChange'],
    ['commit', 'commitRunbookChange'],
    ['search', 'searchRunbooks'],
    ['document', 'readRunbookDocument'],
  ].map(([path, operationId]) => [
    `/assistant/runbooks/${path}`,
    {
      post: {
        operationId,
        requestBody: true,
        responses: writes,
        security: [
          'BrowserSession',
          'CSRFRequestProof',
          ...(path === 'commit' ? ['RecentAuthProof'] : []),
        ],
      },
    },
  ]),
]);
const fields = {
  RunbookDocument: ['id', 'locale', 'title', 'body'],
  RunbookCorpus: ['id', 'version', 'audience', 'documents'],
  RunbookMaterial: ['action', 'expected_version', 'generation', 'digest', 'corpus'],
  RunbookPreview: ['digest', 'confirmation_token', 'expires_at'],
  RunbookCommit: ['material', 'confirmation_token', 'confirmed'],
  RunbookRevision: [
    'id',
    'version',
    'digest',
    'audience',
    'approved',
    'generation',
    'documents',
    'bytes',
  ],
  RunbookCatalog: ['generation', 'version', 'revisions', 'maximum_request_bytes'],
  RunbookReference: ['corpus_id', 'version', 'digest', 'document_id', 'locale', 'generation'],
  RunbookSearch: ['query', 'locale', 'limit', 'cursor'],
  RunbookHit: ['reference', 'title', 'snippet'],
  RunbookResults: ['hits', 'next_cursor'],
};
export function inspectRunbookPolicy(document) {
  const issues = [];
  const schemas = document.components?.schemas ?? {};
  for (const [name, keys] of Object.entries(fields)) {
    const schema = schemas[name];
    if (
      schema?.type !== 'object' ||
      schema.additionalProperties !== false ||
      JSON.stringify(Object.keys(schema.properties ?? {}).sort()) !==
        JSON.stringify([...keys].sort()) ||
      JSON.stringify([...(schema.required ?? [])].sort()) !== JSON.stringify([...keys].sort())
    )
      issues.push(`${name}.closed`);
  }
  for (const [name, field, maximum] of [
    ['RunbookCorpus', 'documents', 32],
    ['RunbookCatalog', 'revisions', 16],
    ['RunbookResults', 'hits', 10],
  ])
    if (schemas[name]?.properties?.[field]?.maxItems !== maximum)
      issues.push(`${name}.${field}.bound`);
  for (const [name, field, maximum] of [
    ['RunbookDocument', 'body', 16384],
    ['RunbookDocument', 'title', 256],
    ['RunbookSearch', 'query', 256],
    ['RunbookSearch', 'cursor', 768],
    ['RunbookHit', 'snippet', 512],
    ['RunbookResults', 'next_cursor', 768],
  ])
    if (schemas[name]?.properties?.[field]?.maxLength !== maximum)
      issues.push(`${name}.${field}.bound`);
  if (schemas.RunbookCommit?.properties?.confirmed?.const !== true)
    issues.push('RunbookCommit.confirmed');
  if (schemas.RunbookSearch?.properties?.limit?.maximum !== 10) issues.push('RunbookSearch.limit');
  return issues;
}
export function runRunbookPolicySelfTests(document) {
  assert.deepEqual(inspectRunbookPolicy(document), []);
  for (const mutate of [
    (copy) => {
      copy.components.schemas.RunbookCommit.properties.confirmed.const = false;
    },
    (copy) => {
      copy.components.schemas.RunbookCorpus.properties.documents.maxItems = 33;
    },
    (copy) => {
      copy.components.schemas.RunbookSearch.properties.owner = { type: 'string', maxLength: 64 };
    },
    (copy) => {
      copy.components.schemas.RunbookDocument.properties.body.maxLength = 65536;
    },
  ]) {
    const copy = structuredClone(document);
    mutate(copy);
    assert.ok(inspectRunbookPolicy(copy).length > 0);
  }
}
