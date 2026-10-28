import { ApiTransportError } from './client';
import { parseAssistantResetEvent, parseAssistantStatusEvent } from './assistant-response';
import type { AssistantResetEvent, AssistantStatusEvent } from './contract';

export type AssistantStatusMessage =
  | { readonly type: 'status'; readonly id: string; readonly data: AssistantStatusEvent }
  | { readonly type: 'reset'; readonly data: AssistantResetEvent };

export interface AssistantStatusDecoder {
  push(chunk: Uint8Array): readonly AssistantStatusMessage[];
  finish(): readonly AssistantStatusMessage[];
  close(): void;
}

const maximumFrameBytes = 16 * 1024;
const maximumChunkBytes = 64 * 1024;
const cursor = (value: unknown): bigint | null => {
  if (
    typeof value !== 'string' ||
    value.length > 19 ||
    value !== value.trim() ||
    !/^(?:0|[1-9][0-9]*)$/u.test(value)
  )
    return null;
  const parsed = BigInt(value);
  return parsed <= 9223372036854775807n ? parsed : null;
};
const bad = (): never => {
  throw new ApiTransportError('unexpected_response');
};

// Only called after the decoded status/reset has passed its flat closed schema.
const uniqueKeys = (text: string) => {
  const keys = new Set<string>();
  for (let index = 0; index < text.length; index += 1) {
    if (text[index] !== '"') continue;
    const start = index++;
    while (index < text.length && text[index] !== '"') {
      if (text[index] === '\\') index += 1;
      index += 1;
    }
    const end = index + 1;
    let next = end;
    while (/[\t\n\r ]/u.test(text[next] ?? 'x')) next += 1;
    if (text[next] !== ':') continue;
    const key: unknown = JSON.parse(text.slice(start, end));
    if (typeof key !== 'string' || keys.has(key)) return bad();
    keys.add(key);
  }
};

// Byte decoder only: no connection, reconnect, rendering, storage or resend.
export const createAssistantStatusDecoder = (
  runId: string,
  afterCursor = '0',
): AssistantStatusDecoder => {
  const initial = cursor(afterCursor);
  if (
    typeof runId !== 'string' ||
    runId.length !== 67 ||
    !/^ar_[0-9a-f]{64}$/u.test(runId) ||
    initial === null
  )
    throw new ApiTransportError('invalid_request');
  const buffer = new Uint8Array(maximumFrameBytes);
  const decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });
  let length = 0,
    lineStart = 0;
  let pendingCR = false,
    firstLine = true,
    closed = false,
    reset = false;
  let lastSequence = initial,
    lastVersion = 0,
    count = 0;
  const fields = new Map<string, string>();

  const erase = () => {
    buffer.fill(0);
    length = 0;
    lineStart = 0;
    fields.clear();
  };
  const dispatch = (messages: AssistantStatusMessage[]) => {
    if (fields.size === 0) {
      erase();
      return;
    }
    if (reset) return bad();
    const kind = fields.get('event'),
      data = fields.get('data'),
      id = fields.get('id');
    if (data === undefined || kind === undefined) return bad();
    const decoded: unknown = JSON.parse(data);
    if (kind === 'assistant.reset.v1') {
      if (id !== undefined && cursor(id) === null) return bad();
      const event = parseAssistantResetEvent(decoded, runId);
      uniqueKeys(data);
      messages.push({ type: 'reset', data: event });
      reset = true;
    } else if (kind === 'assistant.status.v1') {
      const event = parseAssistantStatusEvent(decoded, runId);
      uniqueKeys(data);
      const sequence = BigInt(event.sequence);
      if (
        id !== String(event.sequence) ||
        sequence <= lastSequence ||
        (lastSequence !== 0n && sequence !== lastSequence + 1n) ||
        event.version < lastVersion ||
        count >= 64
      )
        return bad();
      messages.push({ type: 'status', id, data: event });
      lastSequence = sequence;
      lastVersion = event.version;
      count += 1;
    } else return bad();
    erase();
  };
  const line = (end: number, messages: AssistantStatusMessage[]) => {
    let text = decoder.decode(buffer.subarray(lineStart, end));
    if (firstLine) {
      text = text.replace(/^\uFEFF/u, '');
      firstLine = false;
    }
    lineStart = length;
    if (text === '') {
      dispatch(messages);
      return;
    }
    if (text.startsWith(':')) return;
    const colon = text.indexOf(':');
    if (colon < 0) return bad();
    const field = text.slice(0, colon);
    if (!['event', 'data', 'id'].includes(field) || fields.has(field)) return bad();
    const value = text.slice(colon + 1).replace(/^ /u, '');
    if (value.includes('\0')) return bad();
    fields.set(field, value);
  };
  const protect = <T>(work: () => T): T => {
    try {
      return work();
    } catch {
      closed = true;
      erase();
      return bad();
    }
  };
  return {
    push(chunk) {
      return protect(() => {
        if (closed || !(chunk instanceof Uint8Array) || chunk.byteLength > maximumChunkBytes)
          return bad();
        const messages: AssistantStatusMessage[] = [];
        for (const byte of chunk) {
          if (pendingCR && byte !== 10) {
            pendingCR = false;
            line(length - 1, messages);
          }
          if (length >= maximumFrameBytes) return bad();
          buffer[length++] = byte;
          if (byte === 10) {
            line(length - (pendingCR ? 2 : 1), messages);
            pendingCR = false;
          } else pendingCR = byte === 13;
        }
        return messages;
      });
    },
    finish() {
      return protect(() => {
        if (closed) return bad();
        // A final CR can delimit the last complete event without another chunk.
        const messages: AssistantStatusMessage[] = [];
        if (pendingCR) {
          pendingCR = false;
          line(length - 1, messages);
        }
        if (length !== 0 || fields.size !== 0) return bad();
        closed = true;
        erase();
        return messages;
      });
    },
    close() {
      closed = true;
      pendingCR = false;
      erase();
    },
  };
};
