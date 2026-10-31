import {
  createAssistantClient,
  ApiTransportError,
  readAssistantStatus,
  type ApiClient,
  type AssistantRunRequest,
  type AssistantStatusMessage,
} from '@stackplore/api-client';

import { AuthStateError, type AuthStore } from '../auth/auth-store';

export type AssistantOwner = symbol;
export type AssistantApi = ReturnType<typeof createAssistantApi>;

// Private calls pin the initiating session. No retries, persistence or background reads.
export function createAssistantApi(
  client: ApiClient,
  auth: AuthStore,
  statusReader: typeof readAssistantStatus = readAssistantStatus,
) {
  const runs = createAssistantClient(client);
  const owned = async <T>(
    owner: AssistantOwner,
    signal: AbortSignal | undefined,
    lifetime: number,
    action: (signal: AbortSignal) => Promise<T>,
  ): Promise<T> => {
    const current = () => auth.getSessionOwner() === owner && typeof owner === 'symbol';
    if (!current()) throw new AuthStateError();
    if (signal?.aborted === true) throw new DOMException('Private request stopped.', 'AbortError');
    const controller = new AbortController();
    const abort = () => {
      controller.abort();
    };
    const unsubscribe = auth.subscribe(() => {
      if (!current()) abort();
    });
    signal?.addEventListener('abort', abort, { once: true });
    const timer = setTimeout(abort, lifetime);
    let rejectStopped: () => void = () => undefined;
    const stopped = new Promise<never>((_resolve, reject) => {
      rejectStopped = () => {
        reject(new DOMException('Private request stopped.', 'AbortError'));
      };
    });
    controller.signal.addEventListener('abort', rejectStopped, { once: true });
    try {
      const pending = Promise.resolve().then(() => {
        if (controller.signal.aborted)
          throw new DOMException('Private request stopped.', 'AbortError');
        return action(controller.signal);
      });
      const result = await Promise.race([pending, stopped]);
      if (!current()) throw new AuthStateError();
      if (controller.signal.aborted)
        throw new DOMException('Private request stopped.', 'AbortError');
      return result;
    } catch (error) {
      if (!current()) throw new AuthStateError();
      throw error;
    } finally {
      clearTimeout(timer);
      unsubscribe();
      signal?.removeEventListener('abort', abort);
      controller.signal.removeEventListener('abort', rejectStopped);
      controller.abort();
    }
  };
  return {
    getOwner: auth.getSessionOwner,
    subscribeOwner(listener: () => void) {
      let previous = auth.getSessionOwner();
      return auth.subscribe(() => {
        const next = auth.getSessionOwner();
        if (next !== previous) {
          previous = next;
          listener();
        }
      });
    },
    admit(owner: AssistantOwner, input: AssistantRunRequest, key: string, signal?: AbortSignal) {
      return owned(owner, signal, 15_000, (signal) =>
        auth.runSessionAction(
          async (protection) =>
            (
              await runs.admit(input, {
                csrfToken: protection.csrfToken,
                idempotencyKey: key,
                signal,
              })
            ).data,
        ),
      );
    },
    get(owner: AssistantOwner, runId: string, signal?: AbortSignal) {
      return owned(
        owner,
        signal,
        15_000,
        async (signal) => (await runs.get(runId, { signal })).data,
      );
    },
    cancel(owner: AssistantOwner, runId: string, signal?: AbortSignal) {
      return owned(owner, signal, 15_000, (signal) =>
        auth.runSessionAction(
          async (protection) =>
            (await runs.cancel(runId, { csrfToken: protection.csrfToken, signal })).data,
        ),
      );
    },
    citation(owner: AssistantOwner, runId: string, citationId: string, signal?: AbortSignal) {
      return owned(
        owner,
        signal,
        15_000,
        async (signal) => (await runs.citation(runId, citationId, { signal })).data,
      );
    },
    status(
      owner: AssistantOwner,
      runId: string,
      onMessage: (message: AssistantStatusMessage) => unknown,
      signal?: AbortSignal,
      afterCursor = '0',
    ) {
      return owned(owner, signal, 120_000, (signal) =>
        statusReader(runId, {
          signal,
          afterCursor,
          onMessage(message) {
            const checkOwner = () => {
              if (auth.getSessionOwner() !== owner || signal.aborted) throw new AuthStateError();
            };
            checkOwner();
            const returned: unknown = onMessage(message);
            if (returned !== undefined) {
              void Promise.resolve(returned).catch(() => undefined);
              throw new ApiTransportError('invalid_configuration');
            }
            checkOwner();
          },
        }),
      );
    },
  };
}
