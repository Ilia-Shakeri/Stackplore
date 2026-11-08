import {
  assistantHistoryStates,
  type AssistantHistoryResponse,
  type AssistantRunState,
} from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { flushSync } from 'react-dom';
import { useOptionalAssistantApi } from './assistant-context';
import type { AssistantApi, AssistantOwner } from './assistant-api';
import { HistoryMetadataExport } from './HistoryMetadataExport';
import { useAuth } from '../auth/auth-context';
import { useExplorationView } from '../product/exploration-ui';
import { useLocale } from '../../i18n';
import './assistant.css';

const noOwner = () => null;
const noSubscription = () => () => {};
type Scope = {
  readonly api: AssistantApi;
  readonly owner: AssistantOwner;
  readonly state: AssistantRunState;
};
type Navigation = {
  readonly scope: Scope;
  readonly cursors: readonly string[];
  readonly index: number;
};
type Result = {
  readonly scope: Scope;
  readonly cursor: string;
  readonly revision: number;
  readonly data?: AssistantHistoryResponse;
  readonly failed?: boolean;
};

export function AssistantHistoryPage() {
  const api = useOptionalAssistantApi();
  const { snapshot } = useAuth();
  const { t } = useLocale();
  const subscribe = useCallback(
    (listener: () => void) => api?.subscribeOwner(listener) ?? noSubscription(),
    [api],
  );
  const owner = useSyncExternalStore(subscribe, api?.getOwner ?? noOwner, noOwner);
  return (
    <div className="route-page assistant-workspace">
      <header className="route-page__header">
        <h1>{t('assistant.history.title')}</h1>
        <p>{t('assistant.history.notice')}</p>
      </header>
      <a href="/assistant">{t('assistant.history.back')}</a>
      {snapshot.status === 'authenticated' ? (
        api !== null && owner !== null ? (
          <PrivateHistory api={api} owner={owner} />
        ) : (
          <p role="status">{t('assistant.history.unavailable')}</p>
        )
      ) : (
        <p role="status">{t('assistant.workspace.guest')}</p>
      )}
    </div>
  );
}

function PrivateHistory({
  api,
  owner,
}: {
  readonly api: AssistantApi;
  readonly owner: AssistantOwner;
}) {
  const { t, locale, formatDate, formatNumber } = useLocale();
  const view = useExplorationView();
  const selected = view.read('history_state');
  const state: AssistantRunState =
    assistantHistoryStates.find((item) => item === selected) ?? 'completed';
  const scope = useMemo(() => ({ api, owner, state }), [api, owner, state]);
  const [navigation, setNavigation] = useState<Navigation | null>(null);
  const [result, setResult] = useState<Result | null>(null);
  const [revision, setRevision] = useState(0);
  const [suspended, setSuspended] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const focusOnLoad = useRef(false);
  const work = useRef<AbortController | null>(null);
  const cursors = navigation?.scope === scope ? navigation.cursors : [''];
  const index = navigation?.scope === scope ? navigation.index : 0;
  const cursor = cursors[index] ?? '';
  const current =
    !suspended &&
    result?.scope === scope &&
    result.cursor === cursor &&
    result.revision === revision
      ? result
      : null;
  const data = current?.data;

  useEffect(() => {
    setNavigation(null);
  }, [scope]);

  useEffect(() => {
    if (suspended) return;
    const controller = new AbortController();
    work.current = controller;
    setResult(null);
    void api
      .history(owner, { state, limit: 10, ...(cursor === '' ? {} : { cursor }) }, controller.signal)
      .then(
        (data) => {
          if (!controller.signal.aborted && api.getOwner() === owner)
            setResult({ scope, cursor, revision, data });
        },
        () => {
          if (!controller.signal.aborted && api.getOwner() === owner)
            setResult({ scope, cursor, revision, failed: true });
        },
      );
    return () => {
      controller.abort();
      if (work.current === controller) work.current = null;
    };
  }, [api, owner, scope, state, cursor, revision, suspended]);
  useEffect(() => {
    if (data !== undefined && focusOnLoad.current) {
      focusOnLoad.current = false;
      heading.current?.focus();
    }
  }, [data]);
  useEffect(() => {
    const hide = () => {
      work.current?.abort();
      flushSync(() => {
        setSuspended(true);
        setResult(null);
        setNavigation(null);
      });
    };
    const show = () => {
      setSuspended(false);
    };
    window.addEventListener('pagehide', hide);
    window.addEventListener('pageshow', show);
    return () => {
      window.removeEventListener('pagehide', hide);
      window.removeEventListener('pageshow', show);
    };
  }, []);
  const restart = () => {
    focusOnLoad.current = true;
    setNavigation(null);
    setRevision((value) => value + 1);
  };
  return (
    <section className="assistant" aria-labelledby="private-history-heading">
      <h2 id="private-history-heading" ref={heading} tabIndex={-1}>
        {t('assistant.history.records')}
      </h2>
      <div className="assistant__actions">
        <label className="assistant__field">
          <span>{t('assistant.history.filter')}</span>
          <select
            value={state}
            onChange={(event) => {
              focusOnLoad.current = true;
              view.write('history_state', event.currentTarget.value);
            }}
          >
            {assistantHistoryStates.map((item) => (
              <option key={item} value={item}>
                {t(`assistant.history.state.${item}`)}
              </option>
            ))}
          </select>
        </label>
        <Button variant="outline" onClick={restart}>
          {t('assistant.history.refresh')}
        </Button>
      </div>
      {current === null ? <p role="status">{t('product.loading')}</p> : null}
      {current?.failed === true ? <p role="alert">{t('assistant.history.unavailable')}</p> : null}
      {data?.entries.length === 0 ? <p role="status">{t('assistant.history.empty')}</p> : null}
      {data !== undefined && data.entries.length > 0 ? (
        <HistoryMetadataExport
          key={`${locale}:${state}:${String(index)}:${String(revision)}`}
          api={api}
          owner={owner}
          input={{state,limit:10,...(cursor === '' ? {} : {cursor})}}
          entries={data.entries}
        >
        {({selected,busy,select}) => (
        <ol className="assistant-history-list" start={index * 10 + 1}>
          {data.entries.map((entry) => (
            <li key={entry.id}>
              <article className="assistant-history-row">
                <label>
                  <input type="checkbox" checked={selected.includes(entry.id)} disabled={busy}
                    aria-label={`${t('assistant.history.export.select')}: ${entry.id}`}
                    onChange={event => { select(entry.id,event.target.checked); }}/>
                  {t('assistant.history.export.select')}
                </label>
                <h3>
                  {t(
                    entry.goal === 'investigate_incident'
                      ? 'assistant.history.investigation'
                      : 'assistant.history.explanation',
                  )}
                </h3>
                <p>
                  {t(`assistant.history.state.${entry.state}`)} ·{' '}
                  <time dateTime={entry.created_at}>
                    {formatDate(Date.parse(entry.created_at), {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </time>
                </p>
                <dl>
                  <dt>{t('assistant.history.source')}</dt>
                  <dd>
                    <TechnicalText>{entry.source_id}</TechnicalText>
                  </dd>
                  <dt>{t('assistant.history.reads')}</dt>
                  <dd>{formatNumber(entry.tool_calls)}</dd>
                  <dt>{t('assistant.history.responses')}</dt>
                  <dd>{formatNumber(entry.inference_calls)}</dd>
                </dl>
                {entry.usage_uncertain ? <p>{t('assistant.history.uncertain')}</p> : null}
                {entry.resource_id !== null ? (
                  <a
                    href={`/assistant?source=${encodeURIComponent(entry.source_id)}&resource_id=${encodeURIComponent(entry.resource_id)}`}
                  >
                    {t('assistant.history.openResource')}
                  </a>
                ) : (
                  <p>{t('assistant.history.resourceMissing')}</p>
                )}
              </article>
            </li>
          ))}
        </ol>
        )}
        </HistoryMetadataExport>
      ) : null}
      <nav className="assistant__actions" aria-label={t('assistant.history.pages')}>
        <Button
          variant="outline"
          disabled={current === null || index === 0}
          onClick={() => {
            focusOnLoad.current = true;
            setNavigation({ scope, cursors, index: index - 1 });
          }}
        >
          {t('assistant.history.previous')}
        </Button>
        <span>{t('assistant.history.page', { number: formatNumber(index + 1) })}</span>
        <Button
          variant="outline"
          disabled={data?.next_cursor == null || index >= 31}
          onClick={() => {
            if (data?.next_cursor == null || index >= 31) return;
            focusOnLoad.current = true;
            setNavigation({
              scope,
              cursors: [...cursors.slice(0, index + 1), data.next_cursor],
              index: index + 1,
            });
          }}
        >
          {t('assistant.history.next')}
        </Button>
      </nav>
      {index >= 31 ? <p role="status">{t('assistant.history.limit')}</p> : null}
    </section>
  );
}
