import { ApiTransportError, type ApiTransportErrorCode } from './client';
import { createAssistantStatusDecoder, type AssistantStatusMessage } from './assistant-status';

export interface AssistantStatusReadOptions {
  /** Synchronous side effect only; any returned value is rejected. */
  readonly onMessage: (message: AssistantStatusMessage) => unknown;
  readonly signal?: AbortSignal;
  readonly afterCursor?: string;
  readonly fetch?: typeof fetch;
  readonly lifetimeMs?: number;
  readonly idleMs?: number;
}

// One explicitly requested connection. Reset/EOF require a new owner decision;
// this function never reconnects, resubmits a question, fetches content or stores it.
export const readAssistantStatus = async (
  runId: string,
  options: AssistantStatusReadOptions,
): Promise<'ended' | 'reset'> => {
  const lifetime = options.lifetimeMs ?? 120_000;
  const idle = options.idleMs ?? 15_000;
  const afterCursor = options.afterCursor ?? '0';
  if (
    typeof options.onMessage !== 'function' ||
    !Number.isInteger(lifetime) ||
    lifetime < 1 ||
    lifetime > 120_000 ||
    !Number.isInteger(idle) ||
    idle < 1 ||
    idle > 15_000 ||
    idle > lifetime ||
    (options.fetch !== undefined && typeof options.fetch !== 'function')
  )
    throw new ApiTransportError('invalid_configuration');
  const decoder = createAssistantStatusDecoder(runId, afterCursor);
  const ownerSignal = options.signal;
  const onMessage = options.onMessage;
  const fetcher = options.fetch ?? globalThis.fetch;
  const controller = new AbortController();
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  let idleTimer: ReturnType<typeof setTimeout> | undefined;
  let failure: ApiTransportErrorCode = 'network_error';
  let status: number | null = null;
  const abort = () => {
    controller.abort();
  };
  const lifetimeTimer = setTimeout(abort, lifetime);
  ownerSignal?.addEventListener('abort', abort, { once: true });
  const untilStopped = async <T>(pending: Promise<T>): Promise<T> => {
    let rejectStopped: () => void = () => undefined;
    const stopped = new Promise<never>((_, reject) => {
      rejectStopped = () => {
        reject(new Error('status connection stopped'));
      };
    });
    controller.signal.addEventListener('abort', rejectStopped, { once: true });
    try {
      if (controller.signal.aborted) rejectStopped();
      return await Promise.race([pending, stopped]);
    } finally {
      controller.signal.removeEventListener('abort', rejectStopped);
    }
  };
  const check = () => {
    if (ownerSignal?.aborted === true || controller.signal.aborted)
      throw new Error('status connection stopped');
  };
  const armIdle = () => {
    clearTimeout(idleTimer);
    idleTimer = setTimeout(abort, idle);
  };
  const cancelBody = (response: Response) => {
    try {
      void response.body?.cancel().catch(() => undefined);
    } catch {
      /* Already locked or closed. */
    }
  };
  const publish = (messages: readonly AssistantStatusMessage[]): boolean => {
    for (const message of messages) {
      check();
      const returned: unknown = onMessage(message);
      if (returned !== undefined) {
        // A callback must be synchronous; observe a rejected thenable without
        // retaining it or letting its diagnostic become an unhandled rejection.
        void Promise.resolve(returned).catch(() => undefined);
        throw new Error('status callback must be synchronous');
      }
      check();
      if (message.type === 'reset') return true;
    }
    return false;
  };
  try {
    check();
    armIdle();
    const headers: Record<string, string> = { Accept: 'text/event-stream' };
    if (afterCursor !== '0') headers['Last-Event-ID'] = afterCursor;
    const pending = fetcher(`/api/v1/assistant/runs/${runId}/events`, {
      method: 'GET',
      headers,
      signal: controller.signal,
      credentials: 'same-origin',
      mode: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      referrerPolicy: 'no-referrer',
    });
    // A supplied transport may ignore abort and resolve later. Dispose its late
    // body, with no late callback or second request.
    void pending.then(
      (response) => {
        if (controller.signal.aborted) cancelBody(response);
      },
      () => undefined,
    );
    const response = await untilStopped(pending);
    status = response.status;
    check();
    failure = 'unexpected_response';
    if (
      response.status !== 200 ||
      response.redirected ||
      response.type === 'opaque' ||
      response.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() !==
        'text/event-stream' ||
      response.headers.get('Cache-Control')?.trim().toLowerCase() !== 'no-store' ||
      response.body === null
    ) {
      cancelBody(response);
      throw new Error('invalid status response');
    }
    const declared = response.headers.get('Content-Length');
    if (
      declared !== null &&
      (!/^(?:0|[1-9][0-9]*)$/u.test(declared) || Number(declared) > 1024 * 1024)
    ) {
      failure = 'response_too_large';
      cancelBody(response);
      throw new Error('status response bound');
    }
    reader = response.body.getReader();
    let bytes = 0;
    let chunks = 0;
    for (;;) {
      check();
      armIdle();
      const part = await untilStopped(reader.read());
      check();
      if (part.done) return publish(decoder.finish()) ? 'reset' : 'ended';
      if (!(part.value instanceof Uint8Array)) throw new Error('invalid status bytes');
      bytes += part.value.byteLength;
      chunks += 1;
      if (bytes > 1024 * 1024 || chunks > 65_536) {
        failure = 'response_too_large';
        throw new Error('status response bound');
      }
      for (let offset = 0; offset < part.value.byteLength; offset += 64 * 1024) {
        if (publish(decoder.push(part.value.subarray(offset, offset + 64 * 1024)))) return 'reset';
      }
    }
  } catch {
    if (ownerSignal?.aborted === true)
      throw new DOMException('The request was aborted.', 'AbortError');
    if (controller.signal.aborted) failure = 'network_error';
    throw new ApiTransportError(failure, { status });
  } finally {
    clearTimeout(lifetimeTimer);
    clearTimeout(idleTimer);
    ownerSignal?.removeEventListener('abort', abort);
    controller.abort();
    decoder.close();
    if (reader !== undefined) {
      try {
        void reader.cancel().catch(() => undefined);
      } catch {
        /* Reader cleanup stays bounded. */
      }
      try {
        reader.releaseLock();
      } catch {
        /* Pending read is already fenced. */
      }
    }
  }
};
