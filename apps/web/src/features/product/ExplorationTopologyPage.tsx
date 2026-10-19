import type { ProductRelationshipPage, ProductResourcePage } from '@stackplore/api-client';
import { lazy, Suspense, useCallback } from 'react';
import { useLocale } from '../../i18n';
import { useProductApi, useProductSource, useProductView } from './product-context';
import { readEnvironment } from './environment-navigation';
import {
  dataAllowed,
  ObservedPageHeader,
  ObservationNotice,
  PageControls,
  ProductState,
} from './exploration-ui';
import { useCursorPage } from './use-cursor-page';
import { useLiveObservation } from './use-live-observation';
const TopologyGraph = lazy(() =>
  import('./TopologyGraph').then((module) => ({ default: module.TopologyGraph })),
);

export function ProductTopologyPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const environment = readEnvironment(useProductView().search);
  const { t } = useLocale();
  const live = useLiveObservation(selectedSource);
  const loadResources = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductResourcePage> =>
      api.listResources({
        signal,
        limit: 100,
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(environment === undefined ? {} : { environmentId: environment }),
        ...(cursor === undefined ? {} : { cursor }),
      }),
    [api, selectedSource, environment],
  );
  const loadEdges = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductRelationshipPage> =>
      api.listTopology({
        signal,
        limit: 100,
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(cursor === undefined ? {} : { cursor }),
      }),
    [api, selectedSource],
  );
  const resources = useCursorPage(loadResources, live.revision);
  const edges = useCursorPage(loadEdges, live.revision);
  return (
    <ObservedPageHeader
      title={t('route.topology.title')}
      description={t('route.topology.description')}
      streamStatus={live.status}
    >
      {resources.state.kind === 'loading' || edges.state.kind === 'loading' ? (
        <ProductState text={t('product.loading')} />
      ) : null}
      {resources.state.kind === 'error' || edges.state.kind === 'error' ? (
        <ProductState
          text={t('product.error')}
          onRetry={() => {
            resources.retry();
            edges.retry();
          }}
        />
      ) : null}
      {resources.state.kind === 'ready' && edges.state.kind === 'ready' ? (
        <>
          <ObservationNotice data={resources.state.data} />
          <ObservationNotice data={edges.state.data} />
          {dataAllowed(resources.state.data.availability) ? (
            <Suspense fallback={<ProductState text={t('product.loading')} />}>
              <TopologyGraph
                resources={resources.state.data.items}
                relationships={
                  dataAllowed(edges.state.data.availability) ? edges.state.data.items : []
                }
              />
            </Suspense>
          ) : null}
          <section aria-label={t('route.resources.title')}>
            <PageControls
              capped={resources.capped}
              failed={resources.moreFailed}
              hasMore={resources.state.data.page.has_more}
              loading={resources.loadingMore}
              onMore={resources.loadMore}
            />
          </section>
          <section aria-label={t('route.topology.title')}>
            <PageControls
              capped={edges.capped}
              failed={edges.moreFailed}
              hasMore={edges.state.data.page.has_more}
              loading={edges.loadingMore}
              onMore={edges.loadMore}
            />
          </section>
          {resources.state.data.page.has_more || edges.state.data.page.has_more ? (
            <p role="status">{t('product.observation.partial')}</p>
          ) : null}
        </>
      ) : null}
    </ObservedPageHeader>
  );
}
