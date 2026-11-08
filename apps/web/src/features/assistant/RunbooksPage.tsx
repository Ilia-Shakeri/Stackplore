import { Button, Field, Input } from '@stackplore/ui';
import {
  parseRunbookCorpus,
  parseRunbookMaterial,
  type RunbookCatalog,
  type RunbookContent,
  type RunbookProvenance,
  type RunbookMaterial,
  type RunbookPreview,
  type RunbookReference,
  type RunbookResults,
} from '@stackplore/api-client';
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type SyntheticEvent,
} from 'react';
import { flushSync } from 'react-dom';
import { useOptionalAssistantApi } from './assistant-context';
import type { AssistantApi, AssistantOwner } from './assistant-api';
import { useAuth } from '../auth/auth-context';
import { useLocale } from '../../i18n';

const absent = () => null;
const noSubscription = () => () => {};
export function RunbooksPage() {
  const api = useOptionalAssistantApi();
  const { snapshot } = useAuth();
  const { t, locale } = useLocale();
  const subscribe = useCallback(
    (listener: () => void) => api?.subscribeOwner(listener) ?? noSubscription(),
    [api],
  );
  const owner = useSyncExternalStore(subscribe, api?.getOwner ?? absent, absent);
  const [scope, setScope] = useState({ owner, revision: 0 });
  if (scope.owner !== owner) setScope({ owner, revision: scope.revision + 1 });
  return (
    <div className="route-page runbooks-page">
      <header className="route-page__header">
        <h1>{t('runbooks.title')}</h1>
        <p>{t('runbooks.notice')}</p>
      </header>
      <a href="/assistant">{t('assistant.history.back')}</a>
      {snapshot.status === 'authenticated' && api !== null && owner !== null ? (
        <PrivateRunbooks
          key={`${String(scope.revision)}:${snapshot.user.id}:${snapshot.user.role}:${locale}`}
          api={api}
          owner={owner}
          admin={snapshot.user.role === 'administrator'}
        />
      ) : (
        <p role="status">{t('assistant.workspace.guest')}</p>
      )}
    </div>
  );
}

function PrivateRunbooks({
  api,
  owner,
  admin,
}: {
  readonly api: AssistantApi;
  readonly owner: AssistantOwner;
  readonly admin: boolean;
}) {
  const { t, locale, formatNumber } = useLocale();
  const [mode, setMode] = useState<'search' | 'manage'>('search');
  const [query, setQuery] = useState('');
  const [searched, setSearched] = useState('');
  const [results, setResults] = useState<RunbookResults | null>(null);
  const [document, setDocument] = useState<RunbookContent | null>(null);
  const [catalog, setCatalog] = useState<RunbookCatalog | null>(null);
  const [candidate, setCandidate] = useState('');
  const [review, setReview] = useState<{
    material: RunbookMaterial;
    preview: RunbookPreview;
  } | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<'error' | 'saved' | 'uncertain' | 'paused' | null>(null);
  const [paused, setPaused] = useState(false);
  const work = useRef<AbortController | null>(null);
  const serial = useRef(0);
  const heading = useRef<HTMLHeadingElement>(null);
  const documentHeading = useRef<HTMLHeadingElement>(null);
  const reviewHeading = useRef<HTMLHeadingElement>(null);
  const committing = useRef(false);
  const form = useRef<HTMLFormElement>(null);
  const clear = useCallback(() => {
    serial.current++;
    work.current?.abort();
    work.current = null;
    committing.current = false;
    setBusy(false);
    setQuery('');
    setSearched('');
    setResults(null);
    setDocument(null);
    setCatalog(null);
    setCandidate('');
    setReview(null);
    setAcknowledged(false);
    form.current?.reset();
  }, []);
  useEffect(() => {
    clear();
    return () => {
      serial.current++;
      work.current?.abort();
      form.current?.reset();
    };
  }, [clear, owner, locale]);
  useEffect(() => {
    const hide = () => {
      const uncertain = committing.current;
      flushSync(() => {
        clear();
        setPaused(true);
        setMessage(uncertain ? 'uncertain' : 'paused');
      });
    };
    const show = () => {
      setPaused(false);
    };
    window.addEventListener('pagehide', hide);
    window.addEventListener('pageshow', show);
    return () => {
      window.removeEventListener('pagehide', hide);
      window.removeEventListener('pageshow', show);
    };
  }, [clear]);
  useEffect(() => {
    if (document !== null) documentHeading.current?.focus();
  }, [document]);
  useEffect(() => {
    if (review !== null) reviewHeading.current?.focus();
  }, [review]);
  useEffect(() => {
    if (review === null) return;
    const duration = Date.parse(review.preview.expires_at) - Date.now();
    if (!Number.isFinite(duration) || duration <= 0) {
      setReview(null);
      setAcknowledged(false);
      return;
    }
    const timer = setTimeout(() => {
      setReview(null);
      setAcknowledged(false);
      form.current?.reset();
    }, duration);
    return () => {
      clearTimeout(timer);
    };
  }, [review]);
  const perform = async <T,>(
    action: (signal: AbortSignal) => Promise<T>,
    accept: (value: T) => void,
    uncertain = false,
  ) => {
    if (paused || work.current !== null || api.getOwner() !== owner) return;
    const controller = new AbortController();
    work.current = controller;
    const version = ++serial.current;
    setBusy(true);
    setMessage(null);
    const current = () =>
      !controller.signal.aborted && serial.current === version && api.getOwner() === owner;
    try {
      const value = await action(controller.signal);
      if (current()) {
        accept(value);
        heading.current?.focus();
      }
    } catch {
      if (current()) setMessage(uncertain ? 'uncertain' : 'error');
    } finally {
      if (current()) {
        work.current = null;
        committing.current = false;
        setBusy(false);
      }
    }
  };
  const search = (cursor = '') => {
    const text = cursor === '' ? query : searched;
    setDocument(null);
    setResults(null);
    void perform(
      (signal) => api.runbookSearch(owner, { query: text, locale, limit: 10, cursor }, signal),
      (value) => {
        setResults(value);
        setSearched(text);
      },
    );
  };
  const open = (reference: RunbookReference) => {
    setDocument(null);
    void perform((signal) => api.runbookDocument(owner, reference, signal), setDocument);
  };
  const reload = () => {
    setCatalog(null);
    setReview(null);
    setAcknowledged(false);
    void perform((signal) => api.runbookCatalog(owner, signal), setCatalog);
  };
  const preview = (material: RunbookMaterial) => {
    setReview(null);
    setAcknowledged(false);
    form.current?.reset();
    void perform(
      (signal) => api.runbookPreview(owner, material, signal),
      (value) => {
        setReview({ material, preview: value });
      },
    );
  };
  const prepare = () => {
    if (catalog === null) return;
    try {
      if (
        new TextEncoder().encode(candidate).length >
        Math.min(catalog.maximum_request_bytes, 256 * 1024) - 1024
      )
        throw new Error('Material too large');
      const value: unknown = JSON.parse(candidate);
      const corpus = parseRunbookCorpus(value);
      const previous = catalog.revisions.find((value) => value.id === corpus.id);
      preview(
        parseRunbookMaterial({
          action: 'approve',
          expected_version: previous?.version ?? 0,
          generation: catalog.generation,
          digest: '',
          corpus,
        }),
      );
    } catch {
      setReview(null);
      setAcknowledged(false);
      setMessage('error');
    }
  };
  const commit = (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (
      review === null ||
      !acknowledged ||
      busy ||
      Date.now() >= Date.parse(review.preview.expires_at)
    )
      return;
    const data = new FormData(event.currentTarget);
    const password = data.get('current_password');
    event.currentTarget.reset();
    if (typeof password !== 'string' || password.length === 0) return;
    const pending = review;
    committing.current = true;
    setReview(null);
    setAcknowledged(false);
    setCatalog(null);
    void perform(
      (signal) =>
        api.runbookCommit(
          owner,
          pending.material,
          pending.preview.confirmation_token,
          password,
          signal,
        ),
      () => {
        setCandidate('');
        setMessage('saved');
      },
      true,
    );
  };
  const switchMode = (next: 'search' | 'manage') => {
    const uncertain = committing.current;
    clear();
    setMessage(uncertain ? 'uncertain' : null);
    setMode(next);
  };
  return (
    <section className="runbooks-workspace" aria-busy={busy}>
      <nav className="runbooks-tabs" aria-label={t('runbooks.sections')}>
        <Button
          variant={mode === 'search' ? 'primary' : 'secondary'}
          onClick={() => {
            switchMode('search');
          }}
        >
          {t('runbooks.browse')}
        </Button>
        {admin ? (
          <Button
            variant={mode === 'manage' ? 'primary' : 'secondary'}
            onClick={() => {
              switchMode('manage');
            }}
          >
            {t('runbooks.manage')}
          </Button>
        ) : null}
      </nav>
      <h2 ref={heading} tabIndex={-1}>
        {t(mode === 'search' ? 'runbooks.search' : 'runbooks.manage')}
      </h2>
      {message !== null ? (
        <p role={message === 'error' || message === 'uncertain' ? 'alert' : 'status'}>
          {t(`runbooks.${message}`)}
        </p>
      ) : null}
      {busy ? <p role="status">{t('runbooks.loading')}</p> : null}
      {mode === 'search' ? (
        <>
          <form
            className="runbooks-search"
            onSubmit={(event) => {
              event.preventDefault();
              search();
            }}
          >
            <Field label={t('runbooks.query')}>
              <Input
                value={query}
                disabled={busy || paused}
                onChange={(event) => {
                  setQuery(event.target.value);
                  setResults(null);
                  setDocument(null);
                }}
                maxLength={256}
                required
                autoComplete="off"
              />
            </Field>
            <Button type="submit" disabled={busy || paused || query.trim() === ''}>
              {t('runbooks.search')}
            </Button>
          </form>
          <div className="runbooks-columns">
            <section aria-label={t('runbooks.results')}>
              {results !== null && results.hits.length === 0 ? (
                <p role="status">{t('runbooks.empty')}</p>
              ) : null}
              <ul className="runbooks-results">
                {results?.hits.map((hit) => (
                  <li key={`${hit.reference.corpus_id}:${hit.reference.document_id}`}>
                    <Button
                      variant="secondary"
                      disabled={busy || paused}
                      onClick={() => {
                        open(hit.reference);
                      }}
                    >
                      {hit.title}
                    </Button>
                    <p className="runbooks-prose">{hit.snippet}</p>
                    <small>
                      <bdi dir="ltr">
                        {hit.reference.corpus_id} · v{hit.reference.version}
                      </bdi>
                    </small>
                  </li>
                ))}
              </ul>
              {results !== null && results.next_cursor !== '' ? (
                <Button
                  disabled={busy || paused}
                  variant="secondary"
                  onClick={() => {
                    search(results.next_cursor);
                  }}
                >
                  {t('runbooks.next')}
                </Button>
              ) : null}
            </section>
            <article aria-label={t('runbooks.document')} className="runbooks-document">
              {document === null ? (
                <p>{t('runbooks.choose')}</p>
              ) : (
                <>
                  <h3 ref={documentHeading} tabIndex={-1}>
                    {document.document.title}
                  </h3>
                  <p className="runbooks-prose" dir="auto">
                    {document.document.body}
                  </p>
                  <ProvenanceDetails value={document.provenance} />
                  <Button
                    variant="secondary"
                    onClick={() => {
                      setDocument(null);
                    }}
                  >
                    {t('runbooks.close')}
                  </Button>
                </>
              )}
            </article>
          </div>
        </>
      ) : (
        <>
          <p>{t('runbooks.manageNotice')}</p>
          <Button onClick={reload} disabled={busy || paused} variant="secondary">
            {t('runbooks.refresh')}
          </Button>
          {catalog !== null ? (
            <div className="runbooks-columns">
              <section>
                <h3>{t('runbooks.catalog')}</h3>
                {catalog.revisions.length === 0 ? <p>{t('runbooks.noCorpus')}</p> : null}
                <ul className="runbooks-results">
                  {catalog.revisions.map((revision) => (
                    <li key={revision.id}>
                      <bdi dir="ltr">
                        {revision.id} · v{revision.version}
                      </bdi>
                      <p>
                        {t(
                          !revision.approved
                            ? 'runbooks.revoked'
                            : revision.generation === catalog.generation
                              ? 'runbooks.approved'
                              : 'runbooks.stale',
                        )}{' '}
                        · {formatNumber(revision.documents)}
                      </p>
                      <ProvenanceDetails value={revision.provenance} />
                      {revision.approved ? (
                        <Button
                          variant="secondary"
                          disabled={busy || paused}
                          onClick={() => {
                            preview({
                              action: 'revoke',
                              expected_version: revision.version,
                              generation: catalog.generation,
                              digest: revision.digest,
                              corpus: {
                                id: revision.id,
                                version: revision.version,
                                audience: '',
                                documents: [],
                                provenance: null,
                              },
                            });
                          }}
                        >
                          {t('runbooks.revoke')}
                        </Button>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </section>
              <section>
                <h3>{t('runbooks.candidate')}</h3>
                <p>{t('runbooks.format')}</p>
                <label htmlFor="runbook-corpus">{t('runbooks.json')}</label>
                <textarea
                  id="runbook-corpus"
                  dir="ltr"
                  spellCheck={false}
                  autoComplete="off"
                  value={candidate}
                  maxLength={256 * 1024}
                  disabled={busy || paused}
                  onChange={(event) => {
                    setCandidate(event.target.value);
                    setReview(null);
                    setAcknowledged(false);
                    form.current?.reset();
                  }}
                  rows={12}
                />
                <Button onClick={prepare} disabled={busy || paused || candidate === ''}>
                  {t('runbooks.preview')}
                </Button>
              </section>
            </div>
          ) : null}
          {review !== null ? (
            <section className="runbooks-review" aria-label={t('runbooks.review')}>
              <h3 ref={reviewHeading} tabIndex={-1}>
                {t('runbooks.review')}
              </h3>
              <p>
                {t(
                  review.material.action === 'revoke'
                    ? 'runbooks.revokeNotice'
                    : 'runbooks.approveNotice',
                )}
              </p>
              <bdi dir="ltr">
                {review.material.corpus.id} · v{review.material.corpus.version} ·{' '}
                {review.material.corpus.audience}
              </bdi>
              <p>
                <code className="runbooks-digest">{review.preview.digest}</code>
              </p>
              {review.material.corpus.documents.map((value) => (
                <article key={`${value.id}:${value.locale}`}>
                  <h4>
                    {value.title} <bdi dir="ltr">({value.locale})</bdi>
                  </h4>
                  <p className="runbooks-prose" dir="auto">
                    {value.body}
                  </p>
                </article>
              ))}
              {review.material.action === 'approve' ? <ProvenanceDetails value={review.material.corpus.provenance} /> : null}
              <form ref={form} onSubmit={commit}>
                <label>
                  <input
                    type="checkbox"
                    checked={acknowledged}
                    onChange={(event) => {
                      setAcknowledged(event.target.checked);
                    }}
                  />
                  {t('runbooks.confirm')}
                </label>
                <Field label={t('runbooks.password')}>
                  <Input
                    name="current_password"
                    type="password"
                    autoComplete="current-password"
                    required
                  />
                </Field>
                <Button type="submit" disabled={!acknowledged || busy || paused}>
                  {t('runbooks.commit')}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setReview(null);
                    setAcknowledged(false);
                    form.current?.reset();
                  }}
                >
                  {t('runbooks.cancel')}
                </Button>
              </form>
            </section>
          ) : null}
        </>
      )}
    </section>
  );
}

function ProvenanceDetails({value}:{readonly value:RunbookProvenance|null}) {
  const {t}=useLocale();
  if (value === null) return <p>{t('runbooks.provenance.legacy')}</p>;
  return <section aria-label={t('runbooks.provenance.title')}>
    <p>{t(`runbooks.provenance.${value.kind}`)}</p>
    <dl>
      <dt>{t('runbooks.provenance.source')}</dt><dd className="runbooks-prose" dir="auto">{value.source}</dd>
      <dt>{t('runbooks.provenance.license')}</dt><dd className="runbooks-prose" dir="auto">{value.license}</dd>
    </dl>
    <p>{t('runbooks.provenance.notice')}</p>
  </section>;
}
