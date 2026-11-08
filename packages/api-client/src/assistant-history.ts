import { ApiTransportError } from './client';
import { assistantResponseWire } from './assistant-response';
import type {
  AssistantHistoryEntry,
  AssistantHistoryResponse,
  AssistantRunState,
} from './contract';

export const assistantHistoryStates = [
  'queued',
  'running',
  'completed',
  'failed',
  'cancelled',
  'expired',
  'interrupted',
  'uncertain',
] as const;
const cursorPattern = /^[A-Za-z0-9_-]{1,724}\.[A-Za-z0-9_-]{43}$/u;
const runPattern = /^ar_[0-9a-f]{64}$/u;
const sourcePattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const resourcePattern = /^r1_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const { closed, boundedArray, timestamp, fail } = assistantResponseWire;

export interface AssistantHistoryInput {
  readonly state?: AssistantRunState;
  readonly limit?: number;
  readonly cursor?: string;
}

const stateValue = (value: unknown): value is AssistantRunState =>
  typeof value === 'string' && assistantHistoryStates.includes(value as AssistantRunState);
const cursorValue = (value: unknown): value is string =>
  typeof value === 'string' &&
  value.length <= 768 &&
  value === value.trim() &&
  cursorPattern.test(value);
const matches = (value: unknown, pattern: RegExp): value is string =>
  typeof value === 'string' && value.length <= 128 && value === value.trim() && pattern.test(value);

export const assistantHistoryInput = (value: AssistantHistoryInput = {}) => {
  const invalid = (): never => {
    throw new ApiTransportError('invalid_request');
  };
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return invalid();
  const prototype: unknown = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) return invalid();
  const descriptors = Object.getOwnPropertyDescriptors(value);
  if (
    Reflect.ownKeys(descriptors).some(
      (key) => typeof key !== 'string' || !['state', 'limit', 'cursor'].includes(key),
    )
  )
    return invalid();
  for (const descriptor of Object.values(descriptors))
    if (!Object.hasOwn(descriptor, 'value')) return invalid();
  const state: unknown = descriptors.state === undefined ? 'completed' : descriptors.state.value;
  const limit: unknown = descriptors.limit === undefined ? 10 : descriptors.limit.value;
  const cursor: unknown = descriptors.cursor?.value;
  if (
    !stateValue(state) ||
    typeof limit !== 'number' ||
    !Number.isInteger(limit) ||
    limit < 1 ||
    limit > 10 ||
    (descriptors.cursor !== undefined && !cursorValue(cursor))
  )
    return invalid();
  return { state, limit, ...(typeof cursor === 'string' ? { cursor } : {}) };
};

export const parseAssistantHistoryResponse = (
  value: unknown,
  state: AssistantRunState,
  limit = 10,
  priorCursor = '',
): AssistantHistoryResponse => {
  if (!stateValue(state) || !Number.isInteger(limit) || limit < 1 || limit > 10) return fail();
  const page = closed(value, ['entries', 'next_cursor']);
  if (
    page.next_cursor !== null &&
    (!cursorValue(page.next_cursor) || page.next_cursor === priorCursor)
  )
    return fail();
  const entries = boundedArray(page.entries, 0, limit).map((value): AssistantHistoryEntry => {
    const item = closed(value, [
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
    if (
      !matches(item.id, runPattern) ||
      !matches(item.source_id, sourcePattern) ||
      (item.resource_id !== null && !matches(item.resource_id, resourcePattern)) ||
      (item.goal !== 'explain_resource' && item.goal !== 'investigate_incident') ||
      item.state !== state ||
      typeof item.usage_uncertain !== 'boolean'
    )
      return fail();
    if (
      typeof item.tool_calls !== 'number' ||
      !Number.isInteger(item.tool_calls) ||
      item.tool_calls < 0 ||
      item.tool_calls > 12 ||
      typeof item.inference_calls !== 'number' ||
      !Number.isInteger(item.inference_calls) ||
      item.inference_calls < 0 ||
      item.inference_calls > 3
    )
      return fail();
    const created = timestamp(item.created_at, true);
    if (timestamp(item.updated_at, true) < created) return fail();
    const range = closed(item.range, ['from', 'to']);
    const from = timestamp(range.from, true),
      until = timestamp(range.to, true);
    if (until < from || until > created || until - from > 86_400_000_000_000n) return fail();
    return { ...item, range } as AssistantHistoryEntry;
  });
  if (
    new Set(entries.map((item) => item.id)).size !== entries.length ||
    (page.next_cursor !== null && entries.length !== limit)
  )
    return fail();
  for (let index = 1; index < entries.length; index++) {
    const prior = entries[index - 1],
      current = entries[index];
    if (
      prior === undefined ||
      current === undefined ||
      timestamp(current.created_at, true) > timestamp(prior.created_at, true) ||
      (timestamp(current.created_at, true) === timestamp(prior.created_at, true) &&
        current.id >= prior.id)
    )
      return fail();
  }
  return { entries, next_cursor: page.next_cursor as string | null };
};
