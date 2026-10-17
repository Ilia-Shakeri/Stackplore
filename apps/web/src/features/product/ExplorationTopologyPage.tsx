import type { ProductRelationshipPage, ProductResourcePage } from '@stackplore/api-client';
import { lazy, Suspense, useCallback } from 'react';
import { useLocale } from '../../i18n';
import { useProductApi, useProductSource } from './product-context';
import {
  dataAllowed,
  ObservedPageHeader,
  ObservationNotice,
  PageControls,
  ProductState,
} from './exploration-ui';
import { useCursorPage } from './use-cursor-page';
const TopologyGraph = lazy(() =>
  import('./TopologyGraph').then((module) => ({ default: module.TopologyGraph })),
);

export function ProductTopologyPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { t } = useLocale();
  const loadResources = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductResourcePage> =>
      api.listResources({
        signal,
        limit: 100,
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(cursor === undefined ? {} : { cursor }),
      }),
    [api, selectedSource],
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
  const resources = useCursorPage(loadResources);
  const edges = useCursorPage(loadEdges);
  return (
    <ObservedPageHeader
      title={t('route.topology.title')}
      description={t('route.topology.description')}
    >
      {resources.state.kind === 'loading' || edges.state.kind === 'loading' ? (
        <ProductState text={t('product.loading')} />
      ) : null}
      {resources.state.kind === 'error' || edges.state.kind === 'error' ? (
        <ProductState text={t('product.error')} />
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
