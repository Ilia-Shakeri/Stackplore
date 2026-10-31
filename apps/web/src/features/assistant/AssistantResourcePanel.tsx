import type { ProductResource, AssistantWindowMinutes } from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react';
import { flushSync } from 'react-dom';

import { useLocale, type MessageKey } from '../../i18n';
import { useOptionalAuthSnapshot } from '../auth/auth-context';
import type { AssistantApi, AssistantOwner } from './assistant-api';
import { useOptionalAssistantApi } from './assistant-context';
import { AssistantConversation, type ConversationState } from './conversation';
import './assistant.css';

const absentSubscribe = () => () => {};
const absentOwner = () => null;
const windows = [5, 15, 60, 360, 1440] as const;
const stateKeys = {
  idle: 'assistant.state.idle',
  admitting: 'assistant.state.admitting',
  pending: 'assistant.state.pending',
  ready: 'assistant.state.ready',
  unavailable: 'assistant.state.unavailable',
  volatile_lost: 'assistant.state.volatile_lost',
  needs_refresh: 'assistant.state.needs_refresh',
  admission_unknown: 'assistant.state.admission_unknown',
  cancelled: 'assistant.state.cancelled',
  cancel_unknown: 'assistant.state.cancel_unknown',
  cancelling: 'assistant.state.cancelling',
  already_terminal: 'assistant.state.already_terminal',
  expired: 'assistant.state.expired',
  limit: 'assistant.state.limit',
  invalid_question: 'assistant.state.invalid_question',
} as const satisfies Record<ConversationState, MessageKey>;
const sectionKeys = {
  facts: 'assistant.facts',
  hypotheses: 'assistant.hypotheses',
  gaps: 'assistant.gaps',
  next_checks: 'assistant.nextChecks',
} as const satisfies Record<string, MessageKey>;

export function AssistantResourcePanel({ resource }: { readonly resource: ProductResource }) {
  const api = useOptionalAssistantApi();
  const auth = useOptionalAuthSnapshot();
  const subscribe = useCallback(
    (listener: () => void) => api?.subscribeOwner(listener) ?? absentSubscribe(),
    [api],
  );
  const owner = useSyncExternalStore(subscribe, api?.getOwner ?? absentOwner, absentOwner);
  const { t } = useLocale();
  const panelId = useId();
  const [open, setOpen] = useState(false);
  if (api === null || owner === null || auth?.status !== 'authenticated') return null;
  return (
    <section className="assistant" aria-labelledby={`${panelId}-title`}>
      <header className="assistant__header">
        <div>
          <h2 id={`${panelId}-title`}>{t('assistant.title')}</h2>
          <p>{t('assistant.description')}</p>
        </div>
        <Button
          variant="outline"
          aria-expanded={open}
          aria-controls={panelId}
          onClick={() => {
            setOpen((value) => !value);
          }}
        >
          {t(open ? 'assistant.close' : 'assistant.open')}
        </Button>
      </header>
      {open ? (
        <ConversationView api={api} owner={owner} resource={resource} panelId={panelId} />
      ) : null}
    </section>
  );
}

function ConversationView({
  api,
  owner,
  resource,
  panelId,
}: {
  readonly api: AssistantApi;
  readonly owner: AssistantOwner;
  readonly resource: ProductResource;
  readonly panelId: string;
}) {
  const { t, locale, direction, formatNumber } = useLocale();
  const [windowMinutes, setWindowMinutes] = useState<AssistantWindowMinutes>(15);
  const citationAttempt = useRef(0);
  const [citationWarning, setCitationWarning] = useState<symbol | null>(null);
  const conversation = useMemo(
    () =>
      new AssistantConversation(
        api,
        { sourceId: resource.source_id, resourceId: resource.id, windowMinutes, locale },
        owner,
      ),
    [api, owner, resource.source_id, resource.id, windowMinutes, locale],
  );
  const snapshot = useSyncExternalStore(
    conversation.subscribe,
    conversation.getSnapshot,
    conversation.getSnapshot,
  );
  const citationScope = useMemo(() => Symbol(), [conversation, snapshot]);
  useEffect(() => {
    conversation.connect();
    const leave = () => {
      flushSync(() => {
        conversation.erase('expired');
      });
    };
    const visible = () => {
      conversation.checkExpiry();
    };
    window.addEventListener('pagehide', leave);
    window.addEventListener('pageshow', visible);
    document.addEventListener('visibilitychange', visible);
    return () => {
      citationAttempt.current++;
      window.removeEventListener('pagehide', leave);
      window.removeEventListener('pageshow', visible);
      document.removeEventListener('visibilitychange', visible);
      conversation.dispose();
    };
  }, [conversation]);
  const busy = ['admitting', 'pending', 'cancelling'].includes(snapshot.state);
  const uncertain = snapshot.state === 'admission_unknown';
  const citation = (runId: string, citationId: string) => {
    const attempt = ++citationAttempt.current;
    const origin = conversation.getSnapshot();
    setCitationWarning(null);
    void conversation.resolveCitation(runId, citationId).then((result) => {
      if (
        api.getOwner() !== owner ||
        citationAttempt.current !== attempt ||
        conversation.getSnapshot() !== origin
      )
        return;
      if (result === null) setCitationWarning(citationScope);
      else {
        const heading = document.getElementById('resource-detail-name');
        heading?.focus({ preventScroll: true });
        heading?.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'auto' });
      }
    });
  };
  return (
    <div id={panelId} className="assistant__body" dir={direction}>
      <p className="assistant__privacy">{t('assistant.privacy')}</p>
      <dl className="assistant__scope">
        <div>
          <dt>{t('assistant.scope')}</dt>
          <dd>
            <TechnicalText>{resource.source_id}</TechnicalText>
            <span aria-hidden="true"> / </span>
            <TechnicalText>{resource.id}</TechnicalText>
          </dd>
        </div>
      </dl>
      <label className="assistant__field" htmlFor={`${panelId}-window`}>
        <span>{t('assistant.window')}</span>
        <select
          id={`${panelId}-window`}
          value={windowMinutes}
          onChange={(event) => {
            const next = windows.find((value) => String(value) === event.target.value);
            if (next !== undefined) {
              setCitationWarning(null);
              setWindowMinutes(next);
            }
          }}
        >
          {windows.map((value) => (
            <option key={value} value={value}>
              {t('assistant.minutes', { minutes: formatNumber(value) })}
            </option>
          ))}
        </select>
      </label>
      <p id={`${panelId}-state`} role="status" aria-live="polite">
        {t(stateKeys[snapshot.state])}
      </p>
      {citationWarning === citationScope ? (
        <p role="alert">{t('assistant.citationUnavailable')}</p>
      ) : null}
      <ol className="assistant__turns">
        {snapshot.turns.map((turn, index) => (
          <li key={`${turn.response?.run.id ?? 'pending'}-${String(index)}`}>
            <p className="assistant__question" dir="auto">
              {turn.question}
            </p>
            {turn.response?.content_state === 'ready' ? (
              <>
                <h3>{t('assistant.range')}</h3>
                <dl className="assistant__range">
                  <div>
                    <dt>{t('assistant.rangeFrom')}</dt>
                    <dd>
                      <TechnicalText>{turn.response.run.scope.range.from}</TechnicalText>
                    </dd>
                  </div>
                  <div>
                    <dt>{t('assistant.rangeTo')}</dt>
                    <dd>
                      <TechnicalText>{turn.response.run.scope.range.to}</TechnicalText>
                    </dd>
                  </div>
                </dl>
                {turn.response.answer.sections.map((section, sectionIndex) => (
                  <section key={`${section.kind}-${String(sectionIndex)}`}>
                    <h3>{t(sectionKeys[section.kind])}</h3>
                    <p className="assistant__text" dir="auto">
                      {section.text}
                    </p>
                  </section>
                ))}
                {turn.response.citations.map((reference) => (
                  <Button
                    key={reference.id}
                    variant="outline"
                    onClick={() => {
                      citation(turn.response?.run.id ?? '', reference.id);
                    }}
                  >
                    {t('assistant.citation')}
                  </Button>
                ))}
              </>
            ) : null}
          </li>
        ))}
      </ol>
      <p>{t('assistant.turns', { count: formatNumber(snapshot.turns.length) })}</p>
      <form
        autoComplete="off"
        onSubmit={(event) => {
          event.preventDefault();
          void conversation.submit();
        }}
      >
        <label className="assistant__field" htmlFor={`${panelId}-question`}>
          <span>{t('assistant.question')}</span>
          <textarea
            id={`${panelId}-question`}
            name="assistant-question"
            value={snapshot.draft}
            maxLength={8192}
            rows={3}
            autoComplete="off"
            autoCorrect="off"
            autoCapitalize="off"
            spellCheck={false}
            disabled={busy || uncertain || snapshot.state === 'limit'}
            aria-invalid={snapshot.state === 'invalid_question'}
            aria-describedby={`${panelId}-limit ${panelId}-state`}
            onChange={(event) => {
              conversation.setDraft(event.target.value);
            }}
          />
        </label>
        <p id={`${panelId}-limit`}>{t('assistant.questionLimit')}</p>
        <div className="assistant__actions">
          <Button
            type="submit"
            disabled={
              busy || uncertain || snapshot.state === 'limit' || snapshot.draft.trim() === ''
            }
          >
            {t('assistant.send')}
          </Button>
          {uncertain ? (
            <Button
              variant="outline"
              onClick={() => {
                void conversation.retryAdmission();
              }}
            >
              {t('assistant.retry')}
            </Button>
          ) : null}
          {snapshot.state === 'needs_refresh' ? (
            <>
              <Button
                variant="outline"
                onClick={() => {
                  void conversation.refresh();
                }}
              >
                {t('assistant.refresh')}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  void conversation.listen();
                }}
              >
                {t('assistant.listen')}
              </Button>
            </>
          ) : null}
          {busy || uncertain ? (
            <Button
              variant="outline"
              disabled={snapshot.state === 'cancelling'}
              onClick={() => {
                void conversation.cancel();
              }}
            >
              {t('assistant.cancel')}
            </Button>
          ) : (
            <Button
              variant="ghost"
              onClick={() => {
                setCitationWarning(null);
                conversation.erase();
              }}
            >
              {t('assistant.clear')}
            </Button>
          )}
        </div>
      </form>
    </div>
  );
}
