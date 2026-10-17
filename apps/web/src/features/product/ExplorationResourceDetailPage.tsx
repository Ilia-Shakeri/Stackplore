import type { ProductResource, ProductResourceHealth } from '@stackplore/api-client';
import { TechnicalText } from '@stackplore/ui';
import { useEffect, useState } from 'react';
import { useLocale } from '../../i18n';
import { useProductApi } from './product-context';
import { ProductState, type ProductLoadState as LoadState } from './exploration-ui';
import { ResourceMetadataPanel } from './ResourceMetadataPanel';
import { ResourceOperationsPanel } from './ResourceOperationsPanel';
import { ResourceMetricsPanel } from './ResourceMetricsPanel';
interface ProductResourceDetailPageProps {
  readonly id: string;
  readonly kind: string;
  readonly provider: string;
  readonly source: string;
}

interface ResourceDetailData {
  readonly health: ProductResourceHealth | null;
  readonly resource: ProductResource;
}

export function ProductResourceDetailPage({
  id,
  kind,
  provider,
  source,
}: ProductResourceDetailPageProps) {
  const api = useProductApi();
  const { formatDate, t } = useLocale();
  const [state, setState] = useState<LoadState<ResourceDetailData>>({ kind: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void Promise.allSettled([
      api.getResource(id, controller.signal, source),
      api.getResourceHealth(id, controller.signal, source),
    ]).then(
      ([resourceResult, healthResult]) => {
        if (controller.signal.aborted) return;
        if (resourceResult.status !== 'fulfilled') {
          setState({ kind: 'error' });
          return;
        }
        const resource = resourceResult.value;
        const health =
          healthResult.status === 'fulfilled' && healthResult.value.resource_id === id
            ? healthResult.value
            : null;
        if (
          resource.id !== id ||
          resource.source_id !== source ||
          resource.kind !== kind ||
          resource.provider !== provider
        ) {
          setState({ kind: 'error' });
          return;
        }
        setState({ kind: 'ready', data: { health, resource } });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, id, kind, provider, source]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t('route.resourceDetail.title')}</h1>
        <p>{t('route.resourceDetail.description')}</p>
      </header>
      <dl className="resource-identity" aria-label={t('route.resourceDetail.identityLabel')}>
        <Identity label={t('route.resourceDetail.provider')} value={provider} />
        <Identity label={t('route.resourceDetail.kind')} value={kind} />
        <Identity label={t('route.resourceDetail.identifier')} value={id} />
      </dl>
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'ready' ? (
        <section className="resource-detail-card" aria-labelledby="resource-detail-name">
          <div className="resource-detail-card__heading">
            <div>
              <p>{t('product.detail.name')}</p>
              <h2 id="resource-detail-name">{state.data.resource.name}</h2>
            </div>
            <span className={`health-pill health-pill--${state.data.health?.state ?? 'unknown'}`}>
              {healthLabel(state.data.health?.state ?? 'unknown', t)}
            </span>
          </div>
          {state.data.health === null ? (
            <p role="status">{t('explore.detail.healthUnavailable')}</p>
          ) : null}
          <dl>
            <Identity
              label={t('product.detail.scope')}
              value={`${state.data.resource.scope_kind}:${state.data.resource.scope_name ?? '-'}`}
            />
            <div>
              <dt>{t('product.detail.observed')}</dt>
              <dd>
                {formatDate(Date.parse(state.data.resource.observed_at), {
                  dateStyle: 'medium',
                  timeStyle: 'medium',
                })}
              </dd>
            </div>
            <div>
              <dt>{t('product.detail.reasons')}</dt>
              <dd>
                {state.data.health === null
                  ? t('product.source.unavailable')
                  : state.data.health.reason_codes.length === 0
                    ? t('product.detail.noReasons')
                    : state.data.health.reason_codes.join(', ')}
              </dd>
            </div>
          </dl>
        </section>
      ) : null}
      {state.kind === 'ready' ? <ResourceMetadataPanel resource={state.data.resource} /> : null}
      {state.kind === 'ready' ? <ResourceMetricsPanel resourceId={id} sourceId={source} /> : null}
      {state.kind === 'ready' ? <ResourceOperationsPanel resource={state.data.resource} /> : null}
      {state.kind === 'ready' ? (
        <nav className="resource-detail-links" aria-label={t('explore.detail.links')}>
          <a
            href={`/logs?source=${encodeURIComponent(source)}&resource_id=${encodeURIComponent(id)}`}
          >
            {t('explore.detail.logs')}
          </a>
          <a
            href={`/events?source=${encodeURIComponent(source)}&resource_id=${encodeURIComponent(id)}`}
          >
            {t('explore.detail.events')}
          </a>
          <a
            href={`/topology?source=${encodeURIComponent(source)}&topology_selected=${encodeURIComponent(id)}`}
          >
            {t('explore.detail.topology')}
          </a>
          <a href={`/alerts?source=${encodeURIComponent(source)}`}>{t('explore.detail.alerts')}</a>
        </nav>
      ) : null}
    </div>
  );
}

function Identity({ label, value }: { readonly label: string; readonly value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>
        <TechnicalText>{value}</TechnicalText>
      </dd>
    </div>
  );
}

function healthLabel(state: ProductResourceHealth['state'], t: ReturnType<typeof useLocale>['t']) {
  switch (state) {
    case 'healthy':
      return t('product.health.healthy');
    case 'degraded':
      return t('product.health.degraded');
    case 'unhealthy':
      return t('product.health.unhealthy');
    case 'unknown':
      return t('product.health.unknown');
  }
}
