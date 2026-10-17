import type { ProductEventPage } from '@stackplore/api-client';
import { TechnicalText } from '@stackplore/ui';
import { useCallback } from 'react';
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

export function ProductEventsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const stream = useProductStream('events', selectedSource);
  const time = useExplorationTime(stream.revision);
  const view = useExplorationView();
  const resourceId = view.read('resource_id', 128);
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductEventPage> =>
      api.listEvents({
        ...time.range,
        ...(resourceId === '' ? {} : { resourceId }),
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(cursor === undefined ? {} : { cursor }),
        signal,
        limit: 100,
      }),
    [api, selectedSource, resourceId, time.range],
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
                          {event.severity}
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
