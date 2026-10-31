import {
  ApiProblemError,
  ApiTransportError,
  createAssistantIdempotencyKey,
  type AssistantRun,
  type AssistantRunRequest,
  type AssistantRunResponse,
  type AssistantResourceCitation,
  type AssistantWindowMinutes,
} from '@stackplore/api-client';

import { type AssistantApi, type AssistantOwner } from './assistant-api';

export interface ConversationScope {
  readonly sourceId: string;
  readonly resourceId: string;
  readonly windowMinutes: AssistantWindowMinutes;
  readonly locale: 'en' | 'fa';
}
export type ConversationState =
  | 'idle'
  | 'admitting'
  | 'pending'
  | 'ready'
  | 'unavailable'
  | 'volatile_lost'
  | 'needs_refresh'
  | 'admission_unknown'
  | 'cancelled'
  | 'cancel_unknown'
  | 'cancelling'
  | 'already_terminal'
  | 'expired'
  | 'limit'
  | 'invalid_question';
export interface ConversationTurn {
  readonly question: string;
  readonly response?: AssistantRunResponse;
}
export interface ConversationSnapshot {
  readonly state: ConversationState;
  readonly draft: string;
  readonly turns: readonly ConversationTurn[];
  readonly run?: AssistantRun;
}
const lifetime = 30 * 60 * 1000;
const validDraft = (draft: string) =>
  draft.length <= 8192 &&
  !draft.includes('\0') &&
  new TextEncoder().encode(draft).byteLength <= 8192 &&
  !Array.from(draft).some((character) => {
    const unit = character.charCodeAt(0);
    return character.length === 1 && unit >= 0xd800 && unit <= 0xdfff;
  });
const denied = (error: unknown) => {
  const status =
    error instanceof ApiProblemError
      ? error.problem.status
      : error instanceof ApiTransportError
        ? error.status
        : null;
  return status !== null && [401, 403, 404].includes(status);
};
const sameRun = (a: AssistantRun, b: AssistantRun) =>
  a.id === b.id &&
  a.conversation_id === b.conversation_id &&
  a.scope.source_id === b.scope.source_id &&
  a.scope.resource_id === b.scope.resource_id &&
  a.scope.window_minutes === b.scope.window_minutes &&
  a.scope.range.from === b.scope.range.from &&
  a.scope.range.to === b.scope.range.to &&
  a.created_at === b.created_at &&
  a.deadline === b.deadline &&
  b.version >= a.version &&
  (a.state === 'queued' || a.state === 'running' || b.state === a.state) &&
  (b.version > a.version ||
    (b.state === a.state &&
      b.updated_at === a.updated_at &&
      b.usage_uncertain === a.usage_uncertain &&
      b.usage_conservative === a.usage_conservative));

// One mounted resource scope. Text and retry material have no persistence path.
export class AssistantConversation {
  readonly #api: AssistantApi;
  readonly #scope: ConversationScope;
  readonly #owner: AssistantOwner | null;
  readonly #listeners = new Set<() => void>();
  #snapshot: ConversationSnapshot = { state: 'idle', draft: '', turns: [] };
  #epoch = 0;
  #expires = 0;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #unsubscribe: (() => void) | undefined;
  #work: AbortController | undefined;
  #citationWork: AbortController | undefined;
  #disposed = false;
  #conversation: string | undefined;
  #request: { readonly input: AssistantRunRequest; readonly key: string } | undefined;
  #cursor = '0';
  #observedVersion = 0;

  constructor(api: AssistantApi, scope: ConversationScope, owner = api.getOwner()) {
    this.#api = api;
    this.#scope = Object.freeze({ ...scope });
    this.#owner = owner;
    if (this.#owner === null) this.#snapshot = { state: 'unavailable', draft: '', turns: [] };
  }

  getSnapshot = () => this.#snapshot;
  subscribe = (listener: () => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  connect = () => {
    if (this.#unsubscribe !== undefined) return;
    // A remount starts empty; disposed work stays fenced by its old epoch.
    if (this.#disposed) {
      this.#disposed = false;
      this.#publish({ state: 'idle', draft: '', turns: [] });
    }
    this.#unsubscribe = this.#api.subscribeOwner(() => {
      if (this.#api.getOwner() !== this.#owner) this.erase('unavailable');
    });
    if (this.#api.getOwner() !== this.#owner) this.erase('unavailable');
  };

  #publish(snapshot: ConversationSnapshot) {
    this.#snapshot = snapshot;
    for (const listener of this.#listeners) listener();
  }
  #valid(epoch = this.#epoch) {
    if (this.#expires > 0 && Date.now() >= this.#expires) this.erase('expired');
    if (
      this.#owner !== null &&
      this.#api.getOwner() !== this.#owner &&
      (this.#snapshot.state !== 'unavailable' ||
        this.#snapshot.turns.length > 0 ||
        this.#snapshot.draft !== '')
    )
      this.erase('unavailable');
    return (
      !this.#disposed &&
      epoch === this.#epoch &&
      this.#owner !== null &&
      this.#api.getOwner() === this.#owner
    );
  }
  #retain() {
    if (this.#expires !== 0) return;
    this.#expires = Date.now() + lifetime;
    this.#timer = setTimeout(() => {
      this.erase('expired');
    }, lifetime);
  }
  #begin() {
    this.#work?.abort();
    this.#citationWork?.abort();
    this.#citationWork = undefined;
    this.#epoch += 1;
    this.#work = new AbortController();
    return { epoch: this.#epoch, signal: this.#work.signal };
  }
  erase = (state: 'idle' | 'expired' | 'unavailable' = 'idle') => {
    this.#epoch += 1;
    this.#work?.abort();
    this.#work = undefined;
    this.#citationWork?.abort();
    this.#citationWork = undefined;
    clearTimeout(this.#timer);
    this.#timer = undefined;
    this.#expires = 0;
    this.#conversation = undefined;
    this.#request = undefined;
    this.#cursor = '0';
    this.#observedVersion = 0;
    this.#publish({ state, draft: '', turns: [] });
  };
  dispose = () => {
    this.#disposed = true;
    this.#unsubscribe?.();
    this.#unsubscribe = undefined;
    this.#listeners.clear();
    this.erase('expired');
  };
  checkExpiry = () => {
    this.#valid();
  };

  setDraft = (draft: string) => {
    if (
      !this.#valid() ||
      ['admitting', 'pending', 'admission_unknown', 'cancelling'].includes(this.#snapshot.state)
    )
      return;
    if (!validDraft(draft)) {
      this.#publish({ ...this.#snapshot, state: 'invalid_question' });
      return;
    }
    if (draft !== '') this.#retain();
    this.#publish({
      ...this.#snapshot,
      draft,
      state: this.#snapshot.state === 'invalid_question' ? 'idle' : this.#snapshot.state,
    });
  };

  submit = async () => {
    if (
      !this.#valid() ||
      ['admitting', 'pending', 'admission_unknown', 'cancelling'].includes(this.#snapshot.state)
    )
      return;
    if (this.#snapshot.turns.length >= 8) {
      this.#publish({ ...this.#snapshot, state: 'limit' });
      return;
    }
    const question = this.#snapshot.draft;
    if (question.trim() === '' || !validDraft(question)) {
      this.#publish({ ...this.#snapshot, state: 'invalid_question' });
      return;
    }
    try {
      this.#request = {
        key: createAssistantIdempotencyKey(),
        input: Object.freeze({
          source_id: this.#scope.sourceId,
          resource_id: this.#scope.resourceId,
          window_minutes: this.#scope.windowMinutes,
          locale: this.#scope.locale,
          question,
          ...(this.#conversation === undefined ? {} : { conversation_id: this.#conversation }),
        }),
      };
    } catch {
      this.#publish({ ...this.#snapshot, state: 'unavailable' });
      return;
    }
    this.#retain();
    this.#cursor = '0';
    this.#observedVersion = 0;
    this.#publish({
      state: 'admitting',
      draft: '',
      turns: [...this.#snapshot.turns, { question }],
    });
    await this.#admit();
  };

  retryAdmission = async () => {
    if (this.#snapshot.state !== 'admission_unknown' || !this.#valid()) return;
    this.#publish({ ...this.#snapshot, state: 'admitting' });
    await this.#admit();
  };
  async #admit() {
    const request = this.#request,
      owner = this.#owner;
    if (request === undefined || owner === null) return;
    const { epoch, signal } = this.#begin();
    try {
      const { run } = await this.#api.admit(owner, request.input, request.key, signal);
      if (!this.#valid(epoch)) return;
      if (
        run.scope.source_id !== this.#scope.sourceId ||
        run.scope.resource_id !== this.#scope.resourceId ||
        run.scope.window_minutes !== this.#scope.windowMinutes ||
        (this.#conversation !== undefined && run.conversation_id !== this.#conversation)
      )
        throw new ApiTransportError('unexpected_response');
      this.#request = undefined;
      this.#conversation = run.conversation_id;
      this.#publish({ ...this.#snapshot, state: 'pending', run });
      if (run.state === 'queued' || run.state === 'running') await this.listen();
      else await this.refresh();
    } catch (error) {
      if (!this.#valid(epoch)) return;
      if (denied(error)) this.erase('unavailable');
      else if (error instanceof ApiTransportError && error.code === 'invalid_request') {
        this.#request = undefined;
        this.#publish({ ...this.#snapshot, state: 'invalid_question' });
      } else this.#publish({ ...this.#snapshot, state: 'admission_unknown' });
    }
  }

  listen = async () => {
    const run = this.#snapshot.run,
      owner = this.#owner;
    if (
      !this.#valid() ||
      run === undefined ||
      owner === null ||
      !['queued', 'running'].includes(run.state)
    )
      return;
    const { epoch, signal } = this.#begin();
    this.#publish({ ...this.#snapshot, state: 'pending' });
    try {
      await this.#api.status(
        owner,
        run.id,
        (message) => {
          if (!this.#valid(epoch)) return;
          if (message.type === 'reset') this.#cursor = '0';
          else {
            this.#cursor = message.id;
            this.#observedVersion = Math.max(this.#observedVersion, message.data.version);
          }
        },
        signal,
        this.#cursor,
      );
      if (this.#valid(epoch)) await this.refresh();
    } catch (error) {
      if (!this.#valid(epoch)) return;
      if (denied(error)) this.erase('unavailable');
      else this.#publish({ ...this.#snapshot, state: 'needs_refresh' });
    }
  };

  refresh = async () => {
    const run = this.#snapshot.run,
      owner = this.#owner;
    if (!this.#valid() || run === undefined || owner === null) return;
    const { epoch, signal } = this.#begin();
    try {
      const response = await this.#api.get(owner, run.id, signal);
      if (!this.#valid(epoch)) return;
      if (!sameRun(run, response.run) || response.run.version < this.#observedVersion)
        throw new ApiTransportError('unexpected_response');
      if (response.content_state === 'volatile_lost') this.#conversation = undefined;
      const turns = this.#snapshot.turns.map((turn, index) =>
        index === this.#snapshot.turns.length - 1 ? { question: turn.question, response } : turn,
      );
      this.#publish({
        ...this.#snapshot,
        turns,
        run: response.run,
        state:
          response.content_state === 'pending'
            ? 'needs_refresh'
            : response.content_state === 'ready'
              ? 'ready'
              : response.content_state === 'volatile_lost'
                ? 'volatile_lost'
                : 'unavailable',
      });
    } catch (error) {
      if (!this.#valid(epoch)) return;
      if (denied(error)) this.erase('unavailable');
      else this.#publish({ ...this.#snapshot, state: 'needs_refresh' });
    }
  };

  resolveCitation = async (
    runId: string,
    citationId: string,
  ): Promise<AssistantResourceCitation | null> => {
    if (!this.#valid() || this.#owner === null) return null;
    const response = this.#snapshot.turns.find((turn) => turn.response?.run.id === runId)?.response;
    if (
      response?.content_state !== 'ready' ||
      !response.citations.some((citation) => citation.id === citationId)
    )
      return null;
    this.#citationWork?.abort();
    const controller = new AbortController();
    this.#citationWork = controller;
    const epoch = this.#epoch;
    try {
      const citation = await this.#api.citation(this.#owner, runId, citationId, controller.signal);
      if (
        !this.#valid(epoch) ||
        controller.signal.aborted ||
        citation.source_id !== response.run.scope.source_id ||
        citation.resource_id !== response.run.scope.resource_id ||
        citation.range.from !== response.run.scope.range.from ||
        citation.range.to !== response.run.scope.range.to
      )
        return null;
      return citation;
    } catch (error) {
      if (this.#valid(epoch) && denied(error)) this.erase('unavailable');
      return null;
    } finally {
      controller.abort();
      if (this.#citationWork === controller) this.#citationWork = undefined;
    }
  };

  cancel = async () => {
    const run = this.#snapshot.run,
      owner = this.#owner;
    if (!this.#valid() || this.#snapshot.state === 'cancelling') return;
    this.erase();
    if (run === undefined || owner === null) {
      this.#publish({ state: 'cancel_unknown', draft: '', turns: [] });
      return;
    }
    const { epoch, signal } = this.#begin();
    this.#publish({ state: 'cancelling', draft: '', turns: [] });
    try {
      const acknowledged = await this.#api.cancel(owner, run.id, signal);
      if (this.#valid(epoch)) {
        const state = !sameRun(run, acknowledged.run)
          ? 'cancel_unknown'
          : acknowledged.run.state === 'cancelled'
            ? 'cancelled'
            : acknowledged.run.state === 'queued' || acknowledged.run.state === 'running'
              ? 'cancel_unknown'
              : 'already_terminal';
        this.#publish({ state, draft: '', turns: [] });
      }
    } catch (error) {
      if (!this.#valid(epoch)) return;
      this.#publish({
        state: denied(error) ? 'unavailable' : 'cancel_unknown',
        draft: '',
        turns: [],
      });
    }
  };
}
