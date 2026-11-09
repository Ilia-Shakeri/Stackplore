import type {
  ProductResource,
  AssistantWindowMinutes,
  RunbookContent,
} from '@stackplore/api-client';
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
import { IncidentReportView } from './IncidentReportView';
import { ReviewedDocumentPicker, type SelectedGuide } from './ReviewedDocumentPicker';
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
const toolKeys = {
  inventory: 'assistant.tool.inventory',
  health: 'assistant.tool.health',
  metrics: 'assistant.tool.metrics',
  events: 'assistant.tool.events',
  topology: 'assistant.tool.topology',
  logs: 'assistant.tool.logs',
  alert_history: 'assistant.tool.alertHistory',
  workload_history: 'assistant.tool.workloadHistory',
  reviewed_document: 'assistant.tool.reviewedDocument',
} as const satisfies Record<string, MessageKey>;
const availabilityKeys = {
  available: 'assistant.coverage.available',
  stale: 'assistant.coverage.stale',
  unavailable: 'assistant.coverage.unavailable',
  unsupported: 'assistant.coverage.unsupported',
} as const satisfies Record<string, MessageKey>;

export function AssistantResourcePanel({
  resource,
  defaultOpen = false,
}: {
  readonly resource: ProductResource;
  readonly defaultOpen?: boolean;
}) {
  const api = useOptionalAssistantApi();
  const auth = useOptionalAuthSnapshot();
  const subscribe = useCallback(
    (listener: () => void) => api?.subscribeOwner(listener) ?? absentSubscribe(),
    [api],
  );
  const owner = useSyncExternalStore(subscribe, api?.getOwner ?? absentOwner, absentOwner);
  const { t, locale } = useLocale();
  const panelId = useId();
  const [open, setOpen] = useState(defaultOpen);
  const [ownerScope, setOwnerScope] = useState({ owner, revision: 0 });
  if (ownerScope.owner !== owner) setOwnerScope({ owner, revision: ownerScope.revision + 1 });
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
        <ConversationView
          key={`${resource.source_id}:${resource.id}:${locale}:${String(ownerScope.revision)}`}
          api={api}
          owner={owner}
          resource={resource}
          panelId={panelId}
        />
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
  const [goal, setGoal] = useState<'explain_resource' | 'investigate_incident'>('explain_resource');
  const [consentingOwner, setConsentingOwner] = useState<AssistantOwner | null>(null);
  const logConsent = consentingOwner === owner;
  const citationAttempt = useRef(0);
  const [selectedGuide, setSelectedGuide] = useState<SelectedGuide | null>(null);
  const [guideRevision, setGuideRevision] = useState(0);
  const [documentWitness, setDocumentWitness] = useState<RunbookContent | null>(null);
  const documentWork = useRef<AbortController | null>(null);
  const documentHeading = useRef<HTMLHeadingElement>(null);
  const [citationWarning, setCitationWarning] = useState<symbol | null>(null);
  const conversation = useMemo(
    () =>
      new AssistantConversation(
        api,
        {
          sourceId: resource.source_id,
          resourceId: resource.id,
          windowMinutes,
          locale,
          goal,
          logConsent,
        },
        owner,
      ),
    [api, owner, resource.source_id, resource.id, windowMinutes, locale, goal, logConsent],
  );
  const snapshot = useSyncExternalStore(
    conversation.subscribe,
    conversation.getSnapshot,
    conversation.getSnapshot,
  );
  const citationScope = useMemo(() => Symbol(), [conversation, snapshot]);
  useEffect(() => {
    setSelectedGuide(null);
    setDocumentWitness(null);
    setGuideRevision((value) => value + 1);
    conversation.connect();
    const leave = () => {
      flushSync(() => {
        citationAttempt.current++;
        documentWork.current?.abort();
        setSelectedGuide(null);
        setDocumentWitness(null);
        setGuideRevision((value) => value + 1);
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
      documentWork.current?.abort();
      window.removeEventListener('pagehide', leave);
      window.removeEventListener('pageshow', visible);
      document.removeEventListener('visibilitychange', visible);
      conversation.dispose();
    };
  }, [conversation]);
  useEffect(() => {
    if (documentWitness !== null) documentHeading.current?.focus();
  }, [documentWitness]);
  useEffect(() => {
    if (snapshot.state === 'expired') {
      setSelectedGuide(null);
      setDocumentWitness(null);
      setGuideRevision((value) => value + 1);
      documentWork.current?.abort();
    }
  }, [snapshot.state]);
  const busy = ['admitting', 'pending', 'cancelling'].includes(snapshot.state);
  const uncertain = snapshot.state === 'admission_unknown';
  const citation = (runId: string, citationId: string) => {
    const attempt = ++citationAttempt.current;
    const origin = conversation.getSnapshot();
    setCitationWarning(null);
    setDocumentWitness(null);
    documentWork.current?.abort();
    void conversation.resolveCitation(runId, citationId).then((result) => {
      if (
        api.getOwner() !== owner ||
        citationAttempt.current !== attempt ||
        conversation.getSnapshot() !== origin
      )
        return;
      if (result === null) setCitationWarning(citationScope);
      else if (result.kind === 'document' && result.document_reference !== null) {
        const controller = new AbortController();
        documentWork.current = controller;
        void api
          .runbookDocument(owner, result.document_reference, controller.signal)
          .then((content) => {
            if (
              !controller.signal.aborted &&
              api.getOwner() === owner &&
              citationAttempt.current === attempt &&
              conversation.getSnapshot() === origin
            )
              setDocumentWitness(content);
          })
          .catch(() => {
            if (
              !controller.signal.aborted &&
              api.getOwner() === owner &&
              citationAttempt.current === attempt
            )
              setCitationWarning(citationScope);
          });
      } else {
        const heading =
          (result.kind === 'alert' || result.kind === 'workload'
            ? document.getElementById(`incident-evidence-${result.id}`)
            : null) ?? document.getElementById('resource-detail-name');
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
      <label className="assistant__field" htmlFor={`${panelId}-goal`}>
        <span>{t('assistant.mode')}</span>
        <select
          id={`${panelId}-goal`}
          value={goal}
          onChange={(event) => {
            if (
              event.target.value === 'explain_resource' ||
              event.target.value === 'investigate_incident'
            ) {
              setCitationWarning(null);
              setConsentingOwner(null);
              setGoal(event.target.value);
            }
          }}
        >
          <option value="explain_resource">{t('assistant.mode.explain')}</option>
          <option value="investigate_incident">{t('assistant.mode.investigate')}</option>
        </select>
      </label>
      {goal === 'investigate_incident' ? <p>{t('assistant.investigationScope')}</p> : null}
      <label className="assistant__field" htmlFor={`${panelId}-window`}>
        <span>{t('assistant.window')}</span>
        <select
          id={`${panelId}-window`}
          value={windowMinutes}
          onChange={(event) => {
            const next = windows.find((value) => String(value) === event.target.value);
            if (next !== undefined) {
              setCitationWarning(null);
              setConsentingOwner(null);
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
      {goal === 'investigate_incident' ? (
        <label className="assistant__consent" htmlFor={`${panelId}-logs`}>
          <input
            id={`${panelId}-logs`}
            type="checkbox"
            checked={logConsent}
            disabled={windowMinutes > 15 || selectedGuide !== null}
            onChange={(event) => {
              setCitationWarning(null);
              setConsentingOwner(event.target.checked ? owner : null);
            }}
          />
          <span>{t('assistant.logConsent')}</span>
        </label>
      ) : null}
      <ReviewedDocumentPicker
        key={String(guideRevision)}
        api={api}
        owner={owner}
        disabled={busy || uncertain || logConsent}
        selected={selectedGuide}
        onSelect={setSelectedGuide}
      />
      <p id={`${panelId}-state`} role="status" aria-live="polite">
        {t(stateKeys[snapshot.state])}
      </p>
      {citationWarning === citationScope ? (
        <p role="alert">{t('assistant.citationUnavailable')}</p>
      ) : null}
      {documentWitness !== null ? (
        <article className="assistant__guide" aria-label={t('runbooks.document')}>
          <h3 ref={documentHeading} tabIndex={-1}>
            {documentWitness.document.title}
          </h3>
          <p>{t('assistant.guide.witnessNotice')}</p>
          <p className="assistant__text" dir="auto">
            {documentWitness.document.body}
          </p>
          {documentWitness.provenance !== null ? (
            <dl>
              <dt>{t('runbooks.provenance.source')}</dt>
              <dd className="assistant__text" dir="auto">
                {documentWitness.provenance.source}
              </dd>
              <dt>{t('runbooks.provenance.license')}</dt>
              <dd className="assistant__text" dir="auto">
                {documentWitness.provenance.license}
              </dd>
            </dl>
          ) : (
            <p>{t('runbooks.provenance.legacy')}</p>
          )}
          <Button
            variant="outline"
            onClick={() => {
              setDocumentWitness(null);
            }}
          >
            {t('runbooks.close')}
          </Button>
        </article>
      ) : null}
      <ol className="assistant__turns">
        {snapshot.turns.map((turn, index) => (
          <li key={`${turn.response?.run.id ?? 'pending'}-${String(index)}`}>
            <p className="assistant__question" dir="auto">
              {turn.question}
            </p>
            {turn.response?.content_state === 'ready' ? (
              <>
                {turn.response.coverage !== undefined ? (
                  <section aria-label={t('assistant.coverage')}>
                    <h3>{t('assistant.coverage')}</h3>
                    <dl className="assistant__coverage">
                      {turn.response.coverage.map((item) => (
                        <div key={item.tool}>
                          <dt>{t(toolKeys[item.tool])}</dt>
                          <dd>
                            {t(availabilityKeys[item.availability])};{' '}
                            {t('assistant.coverage.records', { count: formatNumber(item.records) })}
                            {item.completeness === 'partial'
                              ? `; ${t('assistant.coverage.partial')}`
                              : ''}
                            {item.more_available ? `; ${t('assistant.coverage.more')}` : ''}
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </section>
                ) : null}
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
                    {t(
                      reference.kind === 'document'
                        ? 'assistant.guide.witness'
                        : 'assistant.citation',
                    )}
                  </Button>
                ))}
                {turn.response.run.goal === 'investigate_incident' ? (
                  <IncidentReportView
                    conversation={conversation}
                    runId={turn.response.run.id}
                    onCitation={(id) => {
                      citation(turn.response?.run.id ?? '', id);
                    }}
                  />
                ) : null}
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
          void conversation.submit(selectedGuide?.reference);
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
