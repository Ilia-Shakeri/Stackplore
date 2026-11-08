import { ApiTransportError, type ApiClient, type ApiRequestProtection } from './client';
import { assistantResponseWire } from './assistant-response';
import type {
  RunbookCatalog,
  RunbookCommit,
  RunbookCorpus,
  RunbookDocument,
  RunbookMaterial,
  RunbookPreview,
  RunbookReference,
  RunbookResults,
  RunbookRevision,
  RunbookSearch,
} from './derived/types.derived';

export type {
  RunbookCatalog,
  RunbookCorpus,
  RunbookDocument,
  RunbookMaterial,
  RunbookPreview,
  RunbookReference,
  RunbookResults,
  RunbookRevision,
  RunbookSearch,
} from './derived/types.derived';
const { closed, boundedArray, fail, timestamp } = assistantResponseWire;
const encoder = new TextEncoder();
const idPattern = /^[a-z][a-z0-9_-]{0,63}$/u;
const digestPattern = /^[a-f0-9]{64}$/u;
const cursorPattern = /^(?:|[A-Za-z0-9_-]{1,724}\.[A-Za-z0-9_-]{43})$/u;
const proofPattern = /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const id = (value: unknown): string =>
  typeof value === 'string' && idPattern.test(value) ? value : fail();
const integer = (value: unknown, minimum: number, maximum = Number.MAX_SAFE_INTEGER): number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum && value <= maximum
    ? value
    : fail();
const locale = (value: unknown): 'en' | 'fa' => (value === 'en' || value === 'fa' ? value : fail());
const digest = (value: unknown): string =>
  typeof value === 'string' && digestPattern.test(value) && value !== '0'.repeat(64)
    ? value
    : fail();
const cursor = (value: unknown): string =>
  typeof value === 'string' && value.length <= 768 && cursorPattern.test(value) ? value : fail();
const audience = (value: unknown): 'administrators' | 'all_users' =>
  value === 'administrators' || value === 'all_users' ? value : fail();
function prose(value: unknown, maximum: number, multiline = false, visible = true): string {
  if (
    typeof value !== 'string' ||
    value.length === 0 ||
    encoder.encode(value).length > maximum ||
    /[\uD800-\uDFFF]/u.test(value)
  )
    return fail();
  // Match the domain: LF/tab are inert body formatting, joiners remain valid Persian text.
  for (const char of value) {
    if (
      (/\p{Cc}/u.test(char) && (!multiline || (char !== '\n' && char !== '\t'))) ||
      (/\p{Cf}/u.test(char) && char !== '\u200c' && char !== '\u200d')
    )
      return fail();
  }
  if (visible && !/\P{White_Space}/u.test(value)) return fail();
  return value;
}
export function parseRunbookDocument(value: unknown): RunbookDocument {
  const item = closed(value, ['id', 'locale', 'title', 'body']);
  return {
    id: id(item.id),
    locale: locale(item.locale),
    title: prose(item.title, 256),
    body: prose(item.body, 16384, true),
  };
}
export function parseRunbookReference(value: unknown): RunbookReference {
  const item = closed(value, [
    'corpus_id',
    'version',
    'digest',
    'document_id',
    'locale',
    'generation',
  ]);
  return {
    corpus_id: id(item.corpus_id),
    version: integer(item.version, 1),
    digest: digest(item.digest),
    document_id: id(item.document_id),
    locale: locale(item.locale),
    generation: integer(item.generation, 0),
  };
}
export function parseRunbookRevision(value: unknown): RunbookRevision {
  const item = closed(value, [
    'id',
    'version',
    'digest',
    'audience',
    'approved',
    'generation',
    'documents',
    'bytes',
  ]);
  if (typeof item.approved !== 'boolean') return fail();
  const documents = integer(item.documents, item.approved ? 1 : 0, 32);
  const bytes = integer(item.bytes, item.approved ? 1 : 0, 524288);
  if (!item.approved && (documents !== 0 || bytes !== 0)) return fail();
  return {
    id: id(item.id),
    version: integer(item.version, 1),
    digest: digest(item.digest),
    audience: audience(item.audience),
    approved: item.approved,
    generation: integer(item.generation, 0),
    documents,
    bytes,
  };
}
export function parseRunbookCatalog(value: unknown): RunbookCatalog {
  const item = closed(value, ['generation', 'version', 'revisions', 'maximum_request_bytes']);
  const revisions = boundedArray(item.revisions, 0, 16).map(parseRunbookRevision);
  for (let i = 1; i < revisions.length; i++)
    if ((revisions[i - 1]?.id ?? '') >= (revisions[i]?.id ?? '')) return fail();
  return {
    generation: integer(item.generation, 0),
    version: integer(item.version, 1),
    revisions,
    maximum_request_bytes: integer(item.maximum_request_bytes, 1024, 1048576),
  };
}
export function parseRunbookPreview(value: unknown): RunbookPreview {
  const item = closed(value, ['digest', 'confirmation_token', 'expires_at']);
  if (
    typeof item.confirmation_token !== 'string' ||
    !proofPattern.test(item.confirmation_token) ||
    typeof item.expires_at !== 'string'
  )
    return fail();
  timestamp(item.expires_at, true);
  return {
    digest: digest(item.digest),
    confirmation_token: item.confirmation_token,
    expires_at: item.expires_at,
  };
}
export function parseRunbookResults(value: unknown, request: RunbookSearch): RunbookResults {
  const item = closed(value, ['hits', 'next_cursor']);
  const hits = boundedArray(item.hits, 0, request.limit).map((value) => {
    const hit = closed(value, ['reference', 'title', 'snippet']);
    const reference = parseRunbookReference(hit.reference);
    if (reference.locale !== request.locale) return fail();
    return {
      reference,
      title: prose(hit.title, 256),
      snippet: prose(hit.snippet, 512, true, false),
    };
  });
  for (let i = 1; i < hits.length; i++) {
    const before = hits[i - 1]?.reference,
      after = hits[i]?.reference;
    if (
      before === undefined ||
      after === undefined ||
      before.corpus_id > after.corpus_id ||
      (before.corpus_id === after.corpus_id && before.document_id >= after.document_id)
    )
      return fail();
  }
  const next_cursor = cursor(item.next_cursor);
  if (next_cursor !== '' && hits.length !== request.limit) return fail();
  return { hits, next_cursor };
}
export function parseRunbookCorpus(value: unknown, revoke = false): RunbookCorpus {
  const item = closed(value, ['id', 'version', 'audience', 'documents']);
  const documents = boundedArray(item.documents, revoke ? 0 : 1, revoke ? 0 : 32).map(
    parseRunbookDocument,
  );
  const keys = new Set<string>();
  let bytes =
    encoder.encode(id(item.id)).length +
    encoder.encode(revoke ? '' : audience(item.audience)).length;
  for (const document of documents) {
    const key = `${document.id}\0${document.locale}`;
    if (keys.has(key)) return fail();
    keys.add(key);
    bytes +=
      encoder.encode(document.id).length +
      encoder.encode(document.locale).length +
      encoder.encode(document.title).length +
      encoder.encode(document.body).length;
  }
  if (bytes > 524288) return fail();
  if (revoke && item.audience !== '') return fail();
  return {
    id: id(item.id),
    version: integer(item.version, 1),
    audience: revoke ? '' : audience(item.audience),
    documents,
  };
}
export function parseRunbookMaterial(value: unknown): RunbookMaterial {
  const item = closed(value, ['action', 'expected_version', 'generation', 'digest', 'corpus']);
  if (item.action !== 'approve' && item.action !== 'revoke') return fail();
  const revoke = item.action === 'revoke';
  const corpus = parseRunbookCorpus(item.corpus, revoke);
  const expected_version = integer(item.expected_version, revoke ? 1 : 0);
  if (corpus.version !== expected_version + (revoke ? 0 : 1)) return fail();
  if (!revoke && item.digest !== '') return fail();
  return {
    action: item.action,
    expected_version,
    generation: integer(item.generation, 0),
    digest: revoke ? digest(item.digest) : '',
    corpus,
  };
}
const parseSearch = (value: unknown): RunbookSearch => {
  const item = closed(value, ['query', 'locale', 'limit', 'cursor']);
  return {
    query: prose(item.query, 256),
    locale: locale(item.locale),
    limit: integer(item.limit, 1, 10),
    cursor: cursor(item.cursor),
  };
};
export function createRunbookClient(client: ApiClient) {
  const request = async <T>(
    path: `/${string}`,
    parse: (value: unknown) => T,
    body: unknown,
    protection: ApiRequestProtection | undefined,
    signal: AbortSignal | undefined,
  ): Promise<T> => {
    const requireActive = () => {
      if (signal?.aborted === true)
        throw new DOMException('Private request stopped.', 'AbortError');
    };
    requireActive();
    const response = await client.request<unknown, unknown>({
      path,
      method: body === undefined ? 'GET' : 'POST',
      ...(body === undefined ? {} : { body }),
      ...(protection === undefined ? {} : { protection }),
      ...(signal === undefined ? {} : { signal }),
    });
    requireActive();
    if (response.status !== 200) throw new ApiTransportError('unexpected_response');
    return parse(response.data);
  };
  return {
    catalog: (signal?: AbortSignal) =>
      request('/assistant/runbooks', parseRunbookCatalog, undefined, undefined, signal),
    preview: (material: RunbookMaterial, protection: ApiRequestProtection, signal?: AbortSignal) =>
      request(
        '/assistant/runbooks/preview',
        parseRunbookPreview,
        parseRunbookMaterial(material),
        protection,
        signal,
      ),
    commit: (input: RunbookCommit, protection: ApiRequestProtection, signal?: AbortSignal) => {
      if (input.confirmed !== true || !proofPattern.test(input.confirmation_token))
        throw new ApiTransportError('invalid_request');
      const material = parseRunbookMaterial(input.material);
      return request(
        '/assistant/runbooks/commit',
        (value) => {
          const result = parseRunbookRevision(value);
          if (
            result.id !== material.corpus.id ||
            result.version !== material.corpus.version ||
            result.generation !== material.generation ||
            result.approved !== (material.action === 'approve')
          )
            return fail();
          return result;
        },
        { material, confirmation_token: input.confirmation_token, confirmed: true },
        protection,
        signal,
      );
    },
    search: (input: RunbookSearch, protection: ApiRequestProtection, signal?: AbortSignal) => {
      const query = parseSearch(input);
      return request(
        '/assistant/runbooks/search',
        (value) => parseRunbookResults(value, query),
        query,
        protection,
        signal,
      );
    },
    document: (input: RunbookReference, protection: ApiRequestProtection, signal?: AbortSignal) => {
      const reference = parseRunbookReference(input);
      return request(
        '/assistant/runbooks/document',
        (value) => {
          const result = parseRunbookDocument(value);
          if (result.id !== reference.document_id || result.locale !== reference.locale)
            return fail();
          return result;
        },
        reference,
        protection,
        signal,
      );
    },
  };
}
