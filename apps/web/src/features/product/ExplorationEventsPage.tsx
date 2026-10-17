import type { ProductEventPage } from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { useCallback, useEffect, useState } from 'react';
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

const severities = ['debug', 'info', 'warning', 'error', 'critical'] as const;

export function ProductEventsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const stream = useProductStream('events', selectedSource);
  const time = useExplorationTime(stream.revision);
  const view = useExplorationView();
  const resourceId = view.read('resource_id', 128);
  const environmentId = view.read('environment_id', 128);
  const search = view.read('event_q');
  const rawSeverity = view.read('severity');
  const severity = severities.find((value) => value === rawSeverity);
  const [draft, setDraft] = useState(search);
  const [environments, setEnvironments] = useState<readonly { id: string; name: string }[]>([]);
  useEffect(() => {
    setDraft(search);
  }, [search]);
  useEffect(() => {
    const controller = new AbortController();
    void api
      .listEnvironments({
        limit: 100,
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        signal: controller.signal,
      })
      .then(
        (page) => {
          if (!controller.signal.aborted) setEnvironments(page.items);
        },
        () => {
          if (!controller.signal.aborted) setEnvironments([]);
        },
      );
    return () => {
      controller.abort();
    };
  }, [api, selectedSource]);
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductEventPage> =>
      api.listEvents({
        ...time.range,
        ...(resourceId === '' ? {} : { resourceId }),
        ...(environmentId === '' ? {} : { environmentId }),
        ...(search === '' ? {} : { search }),
        ...(severity === undefined ? {} : { severity }),
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(cursor === undefined ? {} : { cursor }),
        signal,
        limit: 100,
      }),
    [api, selectedSource, resourceId, environmentId, search, severity, time.range],
  );
  const page = useCursorPage(load);
  return (
    <ObservedPageHeader
      title={t('route.events.title')}
      description={t('route.events.description')}
      streamStatus={stream.status}
    >
      <div className="exploration-filters">
        <TimeRangeControl minutes={time.minutes} select={time.select} />
        <label>
          {t('explore.filter.environment')}
          <select
            value={environmentId}
            onChange={(event) => {
              view.write('environment_id', event.currentTarget.value);
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {environmentId && !environments.some((item) => item.id === environmentId) ? (
              <option value={environmentId}>{environmentId}</option>
            ) : null}
            {environments.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t('explore.logs.severity')}
          <select
            value={severity ?? ''}
            onChange={(event) => {
              view.write('severity', event.currentTarget.value);
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {severities.map((value) => (
              <option key={value} value={value}>
                {t(`explore.events.severity.${value}`)}
              </option>
            ))}
          </select>
        </label>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            view.write('event_q', draft.trim());
          }}
        >
          <label>
            {t('explore.events.search')}
            <input
              maxLength={200}
              value={draft}
              onChange={(event) => {
                setDraft(event.currentTarget.value);
              }}
            />
          </label>
          <Button type="submit">{t('explore.events.apply')}</Button>
        </form>
      </div>
      {page.state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {page.state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {page.state.kind === 'ready' ? (
        <>
          <ObservationNotice data={page.state.data} />
          {dataAllowed(page.state.data.availability) ? (
            <>
              {page.state.data.items.length === 0 ? (
                <ProductState text={t('explore.events.empty')} />
              ) : (
                <div className="event-list">
                  {page.state.data.items.map((event) => (
                    <article key={event.id}>
                      <div>
                        <span className={`severity-chip severity-chip--${event.severity}`}>
                          {t(`explore.events.severity.${event.severity}`)}
                        </span>
                        <time dateTime={event.occurred_at}>
                          {formatDate(Date.parse(event.occurred_at), {
                            dateStyle: 'medium',
                            timeStyle: 'short',
                          })}
                        </time>
                      </div>
                      <strong>{event.summary}</strong>
                      <small>
                        <TechnicalText>{event.reason_code}</TechnicalText>
                      </small>
                      {event.resource_id === undefined ? null : (
                        <a
                          href={`/resources?${selectedSource === undefined ? '' : `source=${encodeURIComponent(selectedSource)}&`}q=${encodeURIComponent(event.resource_id)}`}
                        >
                          {t('product.notification.openResource')}
                        </a>
                      )}
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
