import { ApiTransportError, type ApiClient, type ApiResponse } from './client';
import type {
  AssistantResourceCitation,
  AssistantRunReceipt,
  AssistantRunRequest,
  AssistantRunResponse,
  AssistantWindowMinutes,
} from './contract';

const sourcePattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const resourcePattern = /^r1_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const runPattern = /^ar_[0-9a-f]{64}$/u;
const conversationPattern = /^ac_[0-9a-f]{64}$/u;
const citationPattern = /^ev_[0-9a-f]{64}$/u;
const keyPattern = /^[0-9a-f]{64}$/u;
const bodyFields = new Set([
  'source_id',
  'resource_id',
  'window_minutes',
  'locale',
  'question',
  'conversation_id',
]);

export interface AssistantReadOptions {
  readonly signal?: AbortSignal;
}
export interface AssistantMutationOptions extends AssistantReadOptions {
  readonly csrfToken: string;
}
export interface AssistantAdmissionOptions extends AssistantMutationOptions {
  readonly idempotencyKey: string;
}
export interface AssistantClient {
  admit(
    input: AssistantRunRequest,
    options: AssistantAdmissionOptions,
  ): Promise<ApiResponse<AssistantRunReceipt>>;
  get(runId: string, options?: AssistantReadOptions): Promise<ApiResponse<AssistantRunResponse>>;
  cancel(
    runId: string,
    options: AssistantMutationOptions,
  ): Promise<ApiResponse<AssistantRunReceipt>>;
  citation(
    runId: string,
    citationId: string,
    options?: AssistantReadOptions,
  ): Promise<ApiResponse<AssistantResourceCitation>>;
}

const invalid = (): never => {
  throw new ApiTransportError('invalid_request');
};
const stopped = () => Promise.reject(new DOMException('The request was aborted.', 'AbortError'));
const exact = (value: unknown, pattern: RegExp): value is string =>
  typeof value === 'string' && value.length <= 128 && value === value.trim() && pattern.test(value);

const wellFormed = (value: string) => {
  for (let index = 0; index < value.length; index += 1) {
    const unit = value.charCodeAt(index);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(++index);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) return false;
  }
  return true;
};

const validWindow = (value: unknown): value is AssistantWindowMinutes =>
  value === 5 || value === 15 || value === 60 || value === 360 || value === 1440;

const closedInput = (value: unknown): AssistantRunRequest => {
  if (value === null || typeof value !== 'object') return invalid();
  const input = value as Record<string, unknown>;
  const prototype: unknown = Object.getPrototypeOf(input);
  if (prototype !== Object.prototype && prototype !== null) return invalid();
  const descriptors = Object.getOwnPropertyDescriptors(input);
  if (
    Reflect.ownKeys(descriptors).some(
      (key) =>
        typeof key !== 'string' ||
        !bodyFields.has(key) ||
        !Object.hasOwn(descriptors[key] ?? {}, 'value'),
    )
  )
    return invalid();
  if (
    ['source_id', 'resource_id', 'window_minutes', 'locale', 'question'].some(
      (key) => !Object.hasOwn(descriptors, key),
    )
  )
    return invalid();
  const conversation = Object.hasOwn(input, 'conversation_id') ? input.conversation_id : undefined;
  if (
    !exact(input.source_id, sourcePattern) ||
    !exact(input.resource_id, resourcePattern) ||
    !validWindow(input.window_minutes) ||
    (input.locale !== 'en' && input.locale !== 'fa') ||
    typeof input.question !== 'string' ||
    input.question.length > 8192 ||
    input.question.trim() === '' ||
    input.question.includes('\0') ||
    !wellFormed(input.question) ||
    new TextEncoder().encode(input.question).byteLength > 8192 ||
    (Object.hasOwn(input, 'conversation_id') && !exact(conversation, conversationPattern))
  )
    return invalid();
  return {
    source_id: input.source_id,
    resource_id: input.resource_id,
    window_minutes: input.window_minutes,
    locale: input.locale,
    question: input.question,
    ...(typeof conversation === 'string' ? { conversation_id: conversation } : {}),
  };
};

// Generate only for a new explicit submission. Retrying keeps the original key.
export const createAssistantIdempotencyKey = (): string => {
  const bytes = new Uint8Array(32);
  try {
    globalThis.crypto.getRandomValues(bytes);
    return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
  } catch {
    throw new ApiTransportError('invalid_configuration');
  } finally {
    bytes.fill(0);
  }
};

// This typed request constructor performs no retry, storage, polling or dispatch
// until explicitly called. Server authority and response validation remain required.
export const createAssistantClient = (client: ApiClient): AssistantClient => {
  const candidate: unknown = client;
  if (
    candidate === null ||
    typeof candidate !== 'object' ||
    !('request' in candidate) ||
    typeof candidate.request !== 'function'
  ) {
    throw new ApiTransportError('invalid_configuration');
  }
  const runPath = (runId: string): `/assistant/runs/${string}` => {
    if (!exact(runId, runPattern)) return invalid();
    return `/assistant/runs/${runId}`;
  };
  return {
    admit(input, options) {
      if (options.signal?.aborted === true) return stopped();
      let body: AssistantRunRequest;
      try {
        body = closedInput(input);
      } catch {
        return invalid();
      }
      if (!exact(options.idempotencyKey, keyPattern)) return invalid();
      return client.request<AssistantRunReceipt, AssistantRunRequest>({
        path: '/assistant/runs',
        method: 'POST',
        body,
        protection: { kind: 'session', csrfToken: options.csrfToken },
        headers: { 'Idempotency-Key': options.idempotencyKey },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
    },
    get(runId, options = {}) {
      if (options.signal?.aborted === true) return stopped();
      return client.request<AssistantRunResponse>({
        path: runPath(runId),
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
    },
    cancel(runId, options) {
      if (options.signal?.aborted === true) return stopped();
      return client.request<AssistantRunReceipt>({
        path: `${runPath(runId)}/cancel`,
        method: 'POST',
        protection: { kind: 'session', csrfToken: options.csrfToken },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
    },
    citation(runId, citationId, options = {}) {
      if (options.signal?.aborted === true) return stopped();
      const path = runPath(runId);
      if (!exact(citationId, citationPattern)) return invalid();
      return client.request<AssistantResourceCitation>({
        path: `${path}/citations/${citationId}`,
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
    },
  };
};
