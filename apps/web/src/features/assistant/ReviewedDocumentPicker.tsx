import type { RunbookContent, RunbookReference, RunbookResults } from '@stackplore/api-client';
import { Button, Field, Input, TechnicalText } from '@stackplore/ui';
import { useEffect, useRef, useState } from 'react';
import { useLocale } from '../../i18n';
import type { AssistantApi, AssistantOwner } from './assistant-api';

export interface SelectedGuide {
  readonly reference: RunbookReference;
  readonly title: string;
  readonly excerpt: string;
}

function excerpt(body: string): string {
  let bytes = 0;
  let result = '';
  const encoder = new TextEncoder();
  for (const char of body) {
    bytes += encoder.encode(char).length;
    if (bytes > 256) break;
    result += char;
  }
  return result;
}

export function ReviewedDocumentPicker({
  api,
  owner,
  disabled,
  selected,
  onSelect,
}: {
  readonly api: AssistantApi;
  readonly owner: AssistantOwner;
  readonly disabled: boolean;
  readonly selected: SelectedGuide | null;
  readonly onSelect: (value: SelectedGuide | null) => void;
}) {
  const { t, locale } = useLocale();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<RunbookResults | null>(null);
  const [candidate, setCandidate] = useState<{
    reference: RunbookReference;
    content: RunbookContent;
  } | null>(null);
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const work = useRef<AbortController | null>(null);
  const serial = useRef(0);
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(
    () => () => {
      serial.current++;
      work.current?.abort();
    },
    [api, owner, locale],
  );
  const clear = () => {
    serial.current++;
    work.current?.abort();
    work.current = null;
    setOpen(false);
    setQuery('');
    setResults(null);
    setCandidate(null);
    setConsent(false);
    setBusy(false);
    setFailed(false);
  };
  const perform = async <T,>(
    action: (signal: AbortSignal) => Promise<T>,
    accept: (value: T) => void,
  ) => {
    if (disabled || work.current !== null || api.getOwner() !== owner) return;
    const controller = new AbortController();
    work.current = controller;
    const revision = ++serial.current;
    setBusy(true);
    setFailed(false);
    const current = () =>
      !controller.signal.aborted && serial.current === revision && api.getOwner() === owner;
    try {
      const value = await action(controller.signal);
      if (current()) {
        accept(value);
        heading.current?.focus();
      }
    } catch {
      if (current()) setFailed(true);
    } finally {
      if (current()) {
        work.current = null;
        setBusy(false);
      }
    }
  };
  const search = (cursor = '') => {
    setResults(null);
    setCandidate(null);
    setConsent(false);
    void perform(
      (signal) => api.runbookSearch(owner, { query, locale, limit: 10, cursor }, signal),
      setResults,
    );
  };
  return (
    <section className="assistant__guide" aria-label={t('assistant.guide.title')} aria-busy={busy}>
      <h3 ref={heading} tabIndex={-1}>
        {t('assistant.guide.title')}
      </h3>
      <p>{t('assistant.guide.notice')}</p>
      {selected !== null ? (
        <div>
          <p>
            {selected.title}{' '}
            <TechnicalText>
              {selected.reference.corpus_id} · v{selected.reference.version}
            </TechnicalText>
          </p>
          <p className="assistant__text" dir="auto">
            {selected.excerpt}
          </p>
          <Button
            disabled={disabled}
            variant="outline"
            onClick={() => {
              onSelect(null);
              clear();
            }}
          >
            {t('assistant.guide.remove')}
          </Button>
        </div>
      ) : null}
      {!open ? (
        <Button
          disabled={disabled}
          variant="outline"
          onClick={() => {
            setOpen(true);
          }}
        >
          {t('assistant.guide.choose')}
        </Button>
      ) : (
        <>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              search();
            }}
          >
            <Field label={t('runbooks.query')}>
              <Input
                value={query}
                disabled={busy || disabled}
                maxLength={256}
                required
                autoComplete="off"
                onChange={(event) => {
                  setQuery(event.target.value);
                  setResults(null);
                  setCandidate(null);
                  setConsent(false);
                }}
              />
            </Field>
            <Button type="submit" disabled={disabled || busy || query.trim() === ''}>
              {t('runbooks.search')}
            </Button>
            <Button disabled={disabled} type="button" variant="outline" onClick={clear}>
              {t('runbooks.close')}
            </Button>
          </form>
          {results?.hits.length === 0 ? <p role="status">{t('runbooks.empty')}</p> : null}
          <ul>
            {results?.hits.map((hit) => (
              <li key={`${hit.reference.corpus_id}:${hit.reference.document_id}`}>
                <Button
                  disabled={disabled || busy}
                  variant="outline"
                  onClick={() => {
                    setCandidate(null);
                    setConsent(false);
                    void perform(
                      (signal) => api.runbookDocument(owner, hit.reference, signal),
                      (content) => {
                        setCandidate({ reference: { ...hit.reference }, content });
                      },
                    );
                  }}
                >
                  {hit.title}
                </Button>
              </li>
            ))}
          </ul>
          {results !== null && results.next_cursor !== '' ? (
            <Button
              disabled={disabled || busy}
              variant="outline"
              onClick={() => {
                search(results.next_cursor);
              }}
            >
              {t('runbooks.next')}
            </Button>
          ) : null}
          {candidate !== null ? (
            <article>
              <h4>{candidate.content.document.title}</h4>
              <p className="assistant__text" dir="auto">
                {excerpt(candidate.content.document.body)}
              </p>
              {candidate.content.provenance?.assistance_reviewed === true ? (
                <>
                  <label>
                    <input
                      type="checkbox"
                      checked={consent}
                      disabled={disabled || busy}
                      onChange={(event) => {
                        setConsent(event.target.checked);
                      }}
                    />
                    {t('assistant.guide.consent')}
                  </label>
                  <Button
                    disabled={disabled || busy || !consent}
                    onClick={() => {
                      onSelect({
                        reference: { ...candidate.reference },
                        title: candidate.content.document.title,
                        excerpt: excerpt(candidate.content.document.body),
                      });
                      clear();
                    }}
                  >
                    {t('assistant.guide.include')}
                  </Button>
                </>
              ) : (
                <p role="status">{t('assistant.guide.referenceOnly')}</p>
              )}
            </article>
          ) : null}
        </>
      )}
      {failed ? <p role="alert">{t('runbooks.error')}</p> : null}
    </section>
  );
}
