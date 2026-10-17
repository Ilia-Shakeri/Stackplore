import type {
  ProductEnvironmentPage,
  ProductEventPage,
  ProductMetricResult,
  ProductResourcePage,
} from '@stackplore/api-client';
import { TechnicalText } from '@stackplore/ui';
import { useLocale } from '../../i18n';
import { availabilityLabel, dataAllowed, resourceHref } from './exploration-ui';

export function OverviewInsights({
  environments,
  events,
  metrics,
  resources,
  source,
}: {
  readonly environments: ProductEnvironmentPage;
  readonly events: ProductEventPage | null;
  readonly metrics: ProductMetricResult;
  readonly resources: ProductResourcePage | null;
  readonly source?: string;
}) {
  const { t, formatDate, formatNumber } = useLocale();
  const sourceQuery = source === undefined ? '' : `source=${encodeURIComponent(source)}&`;
  const names = new Map(resources?.items.map((resource) => [resource.id, resource]) ?? []);
  return (
    <>
      <nav className="resource-detail-links" aria-label={t('insight.navigation')}>
        {(
          [
            ['/topology', 'route.topology.title'],
            ['/resources', 'route.resources.title'],
            ['/logs', 'route.logs.title'],
            ['/events', 'route.events.title'],
            ['/alerts', 'route.alerts.title'],
          ] as const
        ).map(([path, label]) => (
          <a key={path} href={`${path}?${sourceQuery}`}>
            {t(label)}
          </a>
        ))}
      </nav>
      <section aria-labelledby="overview-environments">
        <h2 id="overview-environments">{t('insight.environments')}</h2>
        {dataAllowed(environments.availability) ? (
          <>
            <div className="source-grid">
              {environments.items.map((environment) => (
                <article className="source-card" key={environment.id}>
                  <h3>{environment.name}</h3>
                  <p>{availabilityLabel(environment.availability, t)}</p>
                  <a
                    href={`/resources?${sourceQuery}environment_id=${encodeURIComponent(environment.id)}`}
                  >
                    {t('route.resources.title')}
                  </a>
                </article>
              ))}
            </div>
            {environments.items.length === 0 ? <p>{t('insight.empty')}</p> : null}
            {environments.page.has_more || environments.completeness === 'partial' ? (
              <p role="status">{t('product.observation.partial')}</p>
            ) : null}
          </>
        ) : (
          <p>{availabilityLabel(environments.availability, t)}</p>
        )}
      </section>
      <section aria-labelledby="overview-consumers">
        <h2 id="overview-consumers">{t('insight.consumers')}</h2>
        <p>{t('insight.loaded')}</p>
        {dataAllowed(metrics.availability) ? (
          <div className="trend-grid">
            {(['cpu.usage.cores', 'memory.usage.bytes'] as const).map((name) => (
              <div key={name}>
                <h3>
                  {t(name === 'cpu.usage.cores' ? 'product.metric.cpu' : 'product.metric.memory')}
                </h3>
                <ol>
                  {metrics.items
                    .filter((item) => item.name === name)
                    .sort((a, b) => b.value - a.value || a.resource_id.localeCompare(b.resource_id))
                    .slice(0, 5)
                    .map((item) => {
                      const resource = names.get(item.resource_id);
                      return (
                        <li key={item.id}>
                          {resource === undefined ? (
                            <TechnicalText>{item.resource_id}</TechnicalText>
                          ) : (
                            <a href={resourceHref(resource)}>{resource.name}</a>
                          )}
                          :{' '}
                          <TechnicalText>
                            {formatNumber(
                              name === 'memory.usage.bytes' ? item.value / 1048576 : item.value,
                              { maximumFractionDigits: 2 },
                            )}{' '}
                            {t(
                              name === 'memory.usage.bytes'
                                ? 'product.metric.mib'
                                : 'product.metric.cores',
                            )}
                          </TechnicalText>
                        </li>
                      );
                    })}
                </ol>
                {metrics.items.filter((item) => item.name === name).length === 0 ? (
                  <p>{t('insight.empty')}</p>
                ) : null}
              </div>
            ))}
          </div>
        ) : (
          <p>{availabilityLabel(metrics.availability, t)}</p>
        )}
        {metrics.completeness === 'partial' || resources?.page.has_more ? (
          <p role="status">{t('product.observation.partial')}</p>
        ) : null}
      </section>
      <section aria-labelledby="overview-events">
        <h2 id="overview-events">{t('insight.events')}</h2>
        {events === null ? (
          <p>{t('product.source.unavailable')}</p>
        ) : dataAllowed(events.availability) ? (
          <>
            <ol>
              {events.items.slice(0, 10).map((event) => (
                <li key={event.id}>
                  <time dateTime={event.occurred_at}>
                    {formatDate(Date.parse(event.occurred_at), {
                      dateStyle: 'short',
                      timeStyle: 'short',
                    })}
                  </time>{' '}
                  <span>{t(`explore.events.severity.${event.severity}`)}</span> {event.summary}
                </li>
              ))}
            </ol>
            {events.items.length === 0 ? <p>{t('insight.empty')}</p> : null}
            {events.page.has_more || events.completeness === 'partial' ? (
              <p role="status">{t('product.observation.partial')}</p>
            ) : null}
          </>
        ) : (
          <p>{availabilityLabel(events.availability, t)}</p>
        )}
      </section>
    </>
  );
}
