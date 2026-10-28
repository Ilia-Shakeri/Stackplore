import { ApiTransportError } from './client';
import type {
  AssistantResourceCitation,
  AssistantRun,
  AssistantRunReceipt,
  AssistantRunResponse,
  AssistantStatusEvent,
  AssistantResetEvent,
} from './contract';

const runPattern = /^ar_[0-9a-f]{64}$/u;
const citationPattern = /^ev_[0-9a-f]{64}$/u;
const sourcePattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const resourcePattern = /^r1_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const conversationPattern = /^ac_[0-9a-f]{64}$/u;
const states = new Set([
  'queued',
  'running',
  'completed',
  'failed',
  'cancelled',
  'expired',
  'interrupted',
  'uncertain',
]);
const fail = (): never => {
  throw new ApiTransportError('unexpected_response');
};

// Copy only own data fields. Rejected content never enters an error or a log.
const closed = (value: unknown, fields: readonly string[]): Record<string, unknown> => {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return fail();
  const prototype: unknown = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) return fail();
  const descriptors = Object.getOwnPropertyDescriptors(value);
  const keys = Reflect.ownKeys(descriptors);
  if (
    keys.length !== fields.length ||
    keys.some((key) => typeof key !== 'string' || !fields.includes(key))
  )
    return fail();
  const copy: Record<string, unknown> = {};
  for (const field of fields) {
    const descriptor = descriptors[field];
    if (descriptor === undefined || !Object.hasOwn(descriptor, 'value')) return fail();
    copy[field] = descriptor.value as unknown;
  }
  return copy;
};

const matches = (value: unknown, pattern: RegExp): value is string =>
  typeof value === 'string' && value.length <= 128 && value === value.trim() && pattern.test(value);

const timestamp = (value: unknown, microseconds = false): bigint => {
  if (typeof value !== 'string' || value.length > 30) return fail();
  const match =
    /^(?!0000)([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})(?:\.([0-9]{1,9}))?Z$/u.exec(
      value,
    );
  if (match === null || match[1] === undefined) return fail();
  const fraction = match[2] ?? '';
  if (microseconds && fraction.length > 6) return fail();
  const millis = Date.parse(`${match[1]}Z`);
  if (!Number.isFinite(millis) || new Date(millis).toISOString().slice(0, 19) !== match[1])
    return fail();
  return BigInt(millis) * 1000000n + BigInt(fraction.padEnd(9, '0'));
};

const range = (value: unknown) => {
  const result = closed(value, ['from', 'to']);
  const duration = timestamp(result.to, true) - timestamp(result.from, true);
  if (duration <= 0n || duration > 86400000000000n) return fail();
  return { from: result.from as string, to: result.to as string };
};

const parseRun = (value: unknown): AssistantRun => {
  const run = closed(value, [
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
  if (
    !matches(run.id, runPattern) ||
    !matches(run.conversation_id, conversationPattern) ||
    run.goal !== 'explain_resource' ||
    typeof run.state !== 'string' ||
    !states.has(run.state) ||
    typeof run.version !== 'number' ||
    !Number.isInteger(run.version) ||
    run.version < 1 ||
    run.version > 64 ||
    typeof run.usage_uncertain !== 'boolean' ||
    typeof run.usage_conservative !== 'boolean'
  )
    return fail();
  const scope = closed(run.scope, ['source_id', 'resource_id', 'window_minutes', 'range']);
  if (
    !matches(scope.source_id, sourcePattern) ||
    !matches(scope.resource_id, resourcePattern) ||
    typeof scope.window_minutes !== 'number' ||
    ![5, 15, 60, 360, 1440].includes(scope.window_minutes)
  )
    return fail();
  const selectedRange = range(scope.range);
  if (
    timestamp(selectedRange.to) - timestamp(selectedRange.from) !==
    BigInt(scope.window_minutes) * 60000000000n
  )
    return fail();
  const created = timestamp(run.created_at);
  if (timestamp(run.updated_at) < created || timestamp(run.deadline) <= created) return fail();
  return { ...run, scope: { ...scope, range: selectedRange } } as AssistantRun;
};

const parseCitation = (value: unknown): AssistantResourceCitation => {
  const citation = closed(value, [
    'id',
    'kind',
    'source_id',
    'resource_id',
    'observed_at',
    'range',
  ]);
  if (
    !matches(citation.id, citationPattern) ||
    citation.kind !== 'resource' ||
    !matches(citation.source_id, sourcePattern) ||
    !matches(citation.resource_id, resourcePattern)
  )
    return fail();
  timestamp(citation.observed_at);
  return { ...citation, range: range(citation.range) } as AssistantResourceCitation;
};

const inertText = (value: unknown): value is string => {
  if (
    typeof value !== 'string' ||
    value.length > 4096 ||
    value.trim() === '' ||
    !/\P{White_Space}/u.test(value) ||
    value.includes('\0') ||
    /[<>]|\]\(|https?:\/\//u.test(value)
  )
    return false;
  for (let index = 0; index < value.length; index += 1) {
    const unit = value.charCodeAt(index);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(++index);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) return false;
  }
  return new TextEncoder().encode(value).byteLength <= 4096;
};

const boundedArray = (value: unknown, minimum: number, maximum: number): unknown[] => {
  if (!Array.isArray(value) || value.length < minimum || value.length > maximum) return fail();
  // JSON arrays are dense data. Reject holes, custom fields and accessors too.
  const descriptors = Object.getOwnPropertyDescriptors(value);
  if (Reflect.ownKeys(descriptors).length !== value.length + 1) return fail();
  const copy: unknown[] = [];
  for (let index = 0; index < value.length; index += 1) {
    const descriptor = descriptors[String(index)];
    if (descriptor === undefined || !Object.hasOwn(descriptor, 'value')) return fail();
    copy.push(descriptor.value as unknown);
  }
  return copy;
};

// Validation is independent of authorization. The owner still needs fresh server checks.
export const parseAssistantRunReceipt = (
  value: unknown,
  expectedRunId?: string,
): AssistantRunReceipt => {
  const receipt = closed(value, ['run']);
  const run = parseRun(receipt.run);
  if (expectedRunId !== undefined && run.id !== expectedRunId) return fail();
  return { run };
};

export const parseAssistantCitation = (
  value: unknown,
  expectedCitationId: string,
): AssistantResourceCitation => {
  const citation = parseCitation(value);
  if (citation.id !== expectedCitationId) return fail();
  return citation;
};

export const parseAssistantRunResponse = (
  value: unknown,
  expectedRunId: string,
): AssistantRunResponse => {
  // Inspect the discriminator without executing a getter or admitting extra fields.
  if (value === null || typeof value !== 'object') return fail();
  const discriminator = Object.getOwnPropertyDescriptor(value, 'content_state');
  if (discriminator === undefined || !Object.hasOwn(discriminator, 'value')) return fail();
  const state: unknown = discriminator.value;
  const response = closed(
    value,
    state === 'ready' ? ['run', 'content_state', 'answer', 'citations'] : ['run', 'content_state'],
  );
  const run = parseRun(response.run);
  if (run.id !== expectedRunId) return fail();
  if (state === 'pending' && (run.state === 'queued' || run.state === 'running'))
    return { run: { ...run, state: run.state }, content_state: state };
  if (state === 'volatile_lost' && run.state === 'completed')
    return { run: { ...run, state: 'completed' }, content_state: state };
  if (state === 'unavailable') return { run, content_state: state };
  if (state !== 'ready' || run.state !== 'completed') return fail();
  const citations = boundedArray(response.citations, 1, 1).map(parseCitation);
  const citation = citations[0];
  if (
    citation === undefined ||
    citation.source_id !== run.scope.source_id ||
    citation.resource_id !== run.scope.resource_id ||
    citation.range.from !== run.scope.range.from ||
    citation.range.to !== run.scope.range.to
  )
    return fail();
  const answer = closed(response.answer, ['sections']);
  const sections = boundedArray(answer.sections, 1, 12).map((value) => {
    const section = closed(value, ['kind', 'text', 'citations']);
    if (
      typeof section.kind !== 'string' ||
      !['facts', 'hypotheses', 'gaps', 'next_checks'].includes(section.kind) ||
      !inertText(section.text)
    )
      return fail();
    const ids = boundedArray(section.citations, section.kind === 'facts' ? 1 : 0, 64);
    if (new Set(ids).size !== ids.length || ids.some((id) => id !== citation.id)) return fail();
    return { kind: section.kind, text: section.text, citations: ids };
  });
  if (new TextEncoder().encode(JSON.stringify({ sections })).byteLength > 65536) return fail();
  // Every field and cross-reference above has been checked; no untrusted object escapes.
  return { run, content_state: 'ready', answer: { sections }, citations } as AssistantRunResponse;
};

export const parseAssistantStatusEvent = (
  value: unknown,
  expectedRunId: string,
): AssistantStatusEvent => {
  const event = closed(value, ['run_id', 'sequence', 'version', 'state', 'at']);
  if (
    !matches(event.run_id, runPattern) ||
    event.run_id !== expectedRunId ||
    typeof event.sequence !== 'number' ||
    !Number.isInteger(event.sequence) ||
    event.sequence < 1 ||
    event.sequence > 64 ||
    typeof event.version !== 'number' ||
    !Number.isInteger(event.version) ||
    event.version < 1 ||
    event.version > 64 ||
    typeof event.state !== 'string' ||
    !states.has(event.state)
  )
    return fail();
  timestamp(event.at);
  return event as AssistantStatusEvent;
};

export const parseAssistantResetEvent = (
  value: unknown,
  expectedRunId: string,
): AssistantResetEvent => {
  const event = closed(value, ['run_id', 'reason']);
  if (
    !matches(event.run_id, runPattern) ||
    event.run_id !== expectedRunId ||
    typeof event.reason !== 'string' ||
    !['replay_gap', 'cursor_ahead', 'replay_expired', 'queue_overflow'].includes(event.reason)
  )
    return fail();
  return event as AssistantResetEvent;
};
