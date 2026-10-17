import type { ProductLogPage, ProductLogRecord } from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { useCallback, useEffect, useRef, useState, type SyntheticEvent } from 'react';
import { useLocale } from '../../i18n';
import { useProductApi, useProductSource } from './product-context';
import { useProductStream } from './product-stream';
import {
  dataAllowed,
  ObservedPageHeader,
  ObservationNotice,
  PageControls,
  ProductState,
  useExplorationView,
} from './exploration-ui';
import { TimeRangeControl, useExplorationTime } from './exploration-time';
import { useCursorPage } from './use-cursor-page';
import './exploration.css';

const resourcePattern = /^r1_[A-Za-z0-9_-]{43}$/u;
const severities = ['debug', 'info', 'warning', 'error', 'critical'] as const;
const lineText = (line: ProductLogRecord) => `${line.timestamp} ${line.severity} ${line.message}`;

export function ProductLogsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const view = useExplorationView();
  const readViewParameter = view.read;
  const writeViewParameter = view.write;
  const [paused, setPaused] = useState(
    () =>
      view.read('tail') === 'paused' ||
      (view.read('tail') !== 'live' && view.read('from') !== '' && view.read('to') !== ''),
  );
  const [pollRevision, setPollRevision] = useState(0);
  const handledStream = useRef(0);
  const stream = useProductStream('logs', selectedSource, !paused);
  const time = useExplorationTime(pollRevision, !paused);
  const [resource, setResource] = useState(() => readViewParameter('resource_id', 128));
  const [container, setContainer] = useState(() => readViewParameter('container', 256));
  const [previous, setPrevious] = useState(() => readViewParameter('previous') === 'true');
  const [target, setTarget] = useState(() => ({ resource, container, previous }));
  const [query, setQuery] = useState(() => readViewParameter('log_q'));
  const [severity, setSeverity] = useState(() => readViewParameter('severity'));
  const [wrap, setWrap] = useState(() => readViewParameter('wrap') !== 'false');
  const [timestamps, setTimestamps] = useState(() => readViewParameter('timestamps') !== 'false');
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [copyStatus, setCopyStatus] = useState<'copied' | 'error' | null>(null);
  useEffect(() => {
    const params = new URLSearchParams(view.search);
    const next = {
      resource: params.get('resource_id') ?? '',
      container: params.get('container') ?? '',
      previous: params.get('previous') === 'true',
    };
    setResource(next.resource);
    setContainer(next.container);
    setPrevious(next.previous);
    setTarget((current) =>
      current.resource === next.resource &&
      current.container === next.container &&
      current.previous === next.previous
        ? current
        : next,
    );
    setQuery(params.get('log_q') ?? '');
    setSeverity(params.get('severity') ?? '');
    setWrap(params.get('wrap') !== 'false');
    setTimestamps(params.get('timestamps') !== 'false');
    setPaused(
      params.get('tail') === 'paused' ||
        (params.get('tail') !== 'live' && params.has('from') && params.has('to')),
    );
  }, [view.search]);
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductLogPage> =>
      api.listLogs({
        ...time.range,
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(target.resource === '' ? {} : { resourceId: target.resource }),
        ...(target.container === '' ? {} : { container: target.container }),
        ...(target.previous ? { previous: true } : {}),
        ...(cursor === undefined ? {} : { cursor }),
        signal,
        limit: 100,
      }),
    [api, selectedSource, target, time.range],
  );
  const page = useCursorPage(load);
  useEffect(() => {
    if (paused || page.state.kind === 'loading' || page.loadingMore) return;
    const timer = window.setTimeout(() => {
      setPollRevision((current) => current + 1);
    }, 5000);
    return () => {
      window.clearTimeout(timer);
    };
  }, [paused, page.state, page.loadingMore]);
  useEffect(() => {
    if (
      paused ||
      page.state.kind === 'loading' ||
      page.loadingMore ||
      handledStream.current === stream.revision
    )
      return;
    handledStream.current = stream.revision;
    setPollRevision((current) => current + 1);
  }, [paused, page.state.kind, page.loadingMore, stream.revision]);
  useEffect(() => {
    setSelected(new Set());
    setCopyStatus(null);
  }, [load]);
  const records =
    page.state.kind === 'ready' && dataAllowed(page.state.data.availability)
      ? page.state.data.items
      : [];
  const term = query.toLocaleLowerCase();
  const visible = records.filter(
    (record) =>
      (severity === '' || record.severity === severity) &&
      (term === '' || record.message.toLocaleLowerCase().includes(term)),
  );
  const selectedLines = visible.filter((line) => selected.has(line.id));
  const containers = [
    ...new Set(
      records.flatMap((record) =>
        'container' in record && typeof record.container === 'string' ? [record.container] : [],
      ),
    ),
  ];
  const targetSubmit = (event: SyntheticEvent<HTMLFormElement, SubmitEvent>) => {
    event.preventDefault();
    if (resource !== '' && !resourcePattern.test(resource)) return;
    if (resource === '' && (container !== '' || previous)) return;
    setTarget({ resource, container, previous });
    writeViewParameter('resource_id', resource);
    writeViewParameter('container', container);
    writeViewParameter('previous', previous ? 'true' : '');
  };
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(selectedLines.map(lineText).join('\n'));
      setCopyStatus('copied');
    } catch {
      setCopyStatus('error');
    }
  };
  const download = () => {
    const blob = new Blob([visible.map(lineText).join('\n')], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'stackplore-loaded-logs.txt';
    link.click();
    URL.revokeObjectURL(url);
  };
  return (
    <ObservedPageHeader
      title={t('route.logs.title')}
      description={t('route.logs.description')}
      {...(paused ? {} : { streamStatus: stream.status })}
    >
      <form className="log-target" onSubmit={targetSubmit}>
        <label>
          {t('explore.logs.resource')}
          <input
            aria-describedby="log-resource-hint"
            dir="ltr"
            maxLength={46}
            pattern="r1_[A-Za-z0-9_-]{43}"
            value={resource}
            onChange={(event) => {
              setResource(event.target.value);
            }}
          />
        </label>
        <label>
          {t('explore.logs.container')}
          <input
            list="observed-log-containers"
            dir="ltr"
            disabled={resource === ''}
            maxLength={256}
            value={container}
            onChange={(event) => {
              setContainer(event.target.value);
            }}
          />
          <datalist id="observed-log-containers">
            {containers.map((name) => (
              <option key={name} value={name} />
            ))}
          </datalist>
        </label>
        <label className="log-check">
          <input
            type="checkbox"
            disabled={resource === ''}
            checked={previous}
            onChange={(event) => {
              setPrevious(event.target.checked);
            }}
          />
          {t('explore.logs.previous')}
        </label>
        <Button type="submit">{t('explore.logs.apply')}</Button>
      </form>
      <small id="log-resource-hint">{t('explore.logs.allResources')}</small>
      <div className="log-controls">
        <TimeRangeControl minutes={time.minutes} select={time.select} />
        <label>
          {t('explore.logs.search')}
          <input
            type="search"
            maxLength={200}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              writeViewParameter('log_q', event.target.value);
            }}
          />
        </label>
        <label>
          {t('explore.logs.severity')}
          <select
            value={severity}
            onChange={(event) => {
              setSeverity(event.target.value);
              writeViewParameter('severity', event.target.value);
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {severities.map((value) => (
              <option key={value}>{value}</option>
            ))}
          </select>
        </label>
        <label className="log-check">
          <input
            type="checkbox"
            checked={wrap}
            onChange={(event) => {
              setWrap(event.target.checked);
              writeViewParameter('wrap', String(event.target.checked));
            }}
          />
          {t('explore.logs.wrap')}
        </label>
        <label className="log-check">
          <input
            type="checkbox"
            checked={timestamps}
            onChange={(event) => {
              setTimestamps(event.target.checked);
              writeViewParameter('timestamps', String(event.target.checked));
            }}
          />
          {t('explore.logs.timestamps')}
        </label>
        <Button
          onClick={() => {
            const next = !paused;
            setPaused(next);
            view.write('tail', next ? 'paused' : 'live');
            view.write('from', next ? time.range.from : '');
            view.write('to', next ? time.range.to : '');
          }}
        >
          {t(paused ? 'explore.logs.resume' : 'explore.logs.pause')}
        </Button>
      </div>
      {paused ? <p role="status">{t('explore.logs.paused')}</p> : null}
      {page.state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {page.state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {page.state.kind === 'ready' ? (
        <>
          <ObservationNotice data={page.state.data} />
          {dataAllowed(page.state.data.availability) ? (
            <>
              <div className="log-actions">
                <Button
                  disabled={selectedLines.length === 0}
                  onClick={() => {
                    void copy();
                  }}
                >
                  {t('explore.logs.copy')}
                </Button>
                <Button disabled={visible.length === 0} onClick={download}>
                  {t('explore.logs.download')}
                </Button>
              </div>
              <p>{t('explore.logs.downloadHint')}</p>
              {copyStatus === null ? null : (
                <p role="status">
                  {t(copyStatus === 'copied' ? 'explore.logs.copied' : 'explore.logs.copyError')}
                </p>
              )}
              {visible.length === 0 ? (
                <ProductState text={t('explore.logs.empty')} />
              ) : (
                <div
                  className={`log-list log-list--${wrap ? 'wrap' : 'no-wrap'}`}
                  aria-label={t('product.logs.listLabel')}
                >
                  {visible.map((record) => (
                    <article key={record.id}>
                      <div>
                        <label className="log-line-select">
                          <input
                            type="checkbox"
                            aria-label={`${t('explore.logs.select')} ${String(record.sequence)}`}
                            checked={selected.has(record.id)}
                            onChange={(event) => {
                              setSelected((current) => {
                                const next = new Set(current);
                                if (event.target.checked) next.add(record.id);
                                else next.delete(record.id);
                                return next;
                              });
                            }}
                          />
                          <span className={`severity-chip severity-chip--${record.severity}`}>
                            {record.severity}
                          </span>
                        </label>
                        {timestamps ? (
                          <time dateTime={record.timestamp}>
                            {formatDate(Date.parse(record.timestamp), { timeStyle: 'medium' })}
                          </time>
                        ) : null}
                        <TechnicalText>{record.resource_id}</TechnicalText>
                        {record.redacted ? <small>{t('explore.logs.redacted')}</small> : null}
                        {record.truncated ? <small>{t('explore.logs.truncated')}</small> : null}
                      </div>
                      <pre dir="ltr" data-ui-technical>
                        {record.message}
                      </pre>
                    </article>
                  ))}
                </div>
              )}
              <PageControls
                capped={page.capped}
                failed={page.moreFailed}
                hasMore={page.state.data.page.has_more}
                loading={page.loadingMore}
                onMore={page.loadMore}
              />
            </>
          ) : null}
        </>
      ) : null}
    </ObservedPageHeader>
  );
}
