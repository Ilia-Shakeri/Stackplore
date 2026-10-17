import type { AvailabilityState, ProductResource } from '@stackplore/api-client';
import { Button } from '@stackplore/ui';
import type { ReactNode } from 'react';

import { useLocale } from '../../i18n';
import type { ProductStreamStatus } from './product-stream';
import { useProductView } from './product-context';

export type ProductLoadState<T> =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error' }
  | { readonly kind: 'unavailable'; readonly availability: AvailabilityState }
  | { readonly kind: 'ready'; readonly data: T };

export function ProductState({ text }: { readonly text: string }) {
  return (
    <section className="product-state" aria-live="polite">
      <p>{text}</p>
    </section>
  );
}

export function dataAllowed(state: AvailabilityState) {
  return state === 'available' || state === 'stale';
}

export function availabilityLabel(state: AvailabilityState, t: ReturnType<typeof useLocale>['t']) {
  const keys = {
    available: 'product.source.available',
    stale: 'product.source.stale',
    unavailable: 'product.source.unavailable',
    unsupported: 'product.source.unsupported',
    permission_denied: 'product.source.permissionDenied',
  } as const;
  return t(keys[state]);
}

export function ObservationNotice({
  data,
}: {
  readonly data: {
    readonly availability: AvailabilityState;
    readonly completeness: 'complete' | 'partial';
  };
}) {
  const { t } = useLocale();
  if (data.availability === 'available' && data.completeness === 'complete') return null;
  return (
    <p className="observation-notice" role="status">
      {data.availability === 'available' ? null : availabilityLabel(data.availability, t)}{' '}
      {data.completeness === 'partial' ? t('product.observation.partial') : null}
    </p>
  );
}

export function PageControls({
  capped,
  hasMore,
  loading,
  failed,
  onMore,
}: {
  readonly capped: boolean;
  readonly hasMore: boolean;
  readonly loading: boolean;
  readonly failed: boolean;
  readonly onMore: () => Promise<void>;
}) {
  const { t } = useLocale();
  return (
    <div className="exploration-pagination">
      {failed ? <p role="alert">{t('explore.page.error')}</p> : null}
      {capped ? (
        <p role="status">{t('explore.page.limit')}</p>
      ) : hasMore ? (
        <Button
          loading={loading}
          onClick={() => {
            void onMore();
          }}
        >
          {t('explore.page.more')}
        </Button>
      ) : null}
    </div>
  );
}

export function ProductStreamBadge({ status }: { readonly status: ProductStreamStatus }) {
  const { t } = useLocale();
  return (
    <span className={`stream-state stream-state--${status}`} role="status">
      <span aria-hidden="true" />
      {t(`product.stream.${status}`)}
    </span>
  );
}

export function ObservedPageHeader({
  children,
  description,
  streamStatus,
  title,
}: {
  readonly children: ReactNode;
  readonly description: string;
  readonly title: string;
  readonly streamStatus?: ProductStreamStatus;
}) {
  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{title}</h1>
        <p>{description}</p>
        {streamStatus === undefined ? null : <ProductStreamBadge status={streamStatus} />}
      </header>
      {children}
    </div>
  );
}

export function resourceHref(resource: ProductResource) {
  return `/resources/${encodeURIComponent(resource.source_id)}/${encodeURIComponent(resource.provider)}/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.id)}?source=${encodeURIComponent(resource.source_id)}`;
}

export function readViewParameter(name: string, maximum = 200) {
  return typeof window === 'undefined'
    ? ''
    : (new URLSearchParams(window.location.search).get(name)?.slice(0, maximum) ?? '');
}

export function writeViewParameter(name: string, value: string) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (value === '') url.searchParams.delete(name);
  else url.searchParams.set(name, value);
  window.history.replaceState(window.history.state, '', url);
}

export function useExplorationView() {
  const view = useProductView();
  return {
    search: view.search,
    read: (name: string, maximum = 200) =>
      new URLSearchParams(view.search).get(name)?.slice(0, maximum) ?? '',
    write: view.selectParameter,
  };
}
