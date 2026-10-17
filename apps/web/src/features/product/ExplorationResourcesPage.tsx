import type { ProductResourcePage, SourceSummary } from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { useCallback, useEffect, useState, type SyntheticEvent } from 'react';

import { useLocale } from '../../i18n';
import { useProductApi, useProductSource } from './product-context';
import type { ResourceReadOptions } from './product-api';
import {
  availabilityLabel,
  dataAllowed,
  ObservationNotice,
  PageControls,
  ProductState,
  resourceHref,
  useExplorationView,
} from './exploration-ui';
import { useCursorPage } from './use-cursor-page';
import './exploration.css';

type Filters = Pick<
  ResourceReadOptions,
  | 'kind'
  | 'environmentId'
  | 'health'
  | 'availability'
  | 'scopeKind'
  | 'scopeName'
  | 'provider'
  | 'sort'
  | 'direction'
>;
const healthValues = ['healthy', 'degraded', 'unhealthy', 'unknown'] as const;
const availabilityValues = [
  'available',
  'stale',
  'unavailable',
  'unsupported',
  'permission_denied',
] as const;
const providerValues = ['demo', 'docker', 'kubernetes'] as const;
const scopeValues = ['global', 'project', 'namespace'] as const;
function initialFilters(search: string): Filters {
  const read = (name: string) => new URLSearchParams(search).get(name) ?? '';
  const health = read('health');
  const availability = read('availability');
  const provider = read('provider');
  const scope = read('scope_kind');
  const sort = read('sort');
  return {
    kind: read('kind'),
    environmentId: read('environment_id'),
    scopeName: read('scope_name'),
    ...(healthValues.some((value) => value === health)
      ? { health: health as (typeof healthValues)[number] }
      : {}),
    ...(availabilityValues.some((value) => value === availability)
      ? { availability: availability as (typeof availabilityValues)[number] }
      : {}),
    ...(providerValues.some((value) => value === provider)
      ? { provider: provider as (typeof providerValues)[number] }
      : {}),
    ...(scopeValues.some((value) => value === scope)
      ? { scopeKind: scope as (typeof scopeValues)[number] }
      : {}),
    sort: sort === 'kind' || sort === 'observed_at' ? sort : 'name',
    direction: read('direction') === 'desc' ? 'desc' : 'asc',
  };
}

export function ProductSearchPage({ query }: { readonly query: string }) {
  return <ProductResourcesPage initialQuery={query} searchMode />;
}

export function ProductResourcesPage({
  initialQuery,
  searchMode = false,
}: {
  readonly initialQuery?: string;
  readonly searchMode?: boolean;
} = {}) {
  const api = useProductApi();
  const { selectedSource, selectSource, query, selectQuery } = useProductSource();
  const { t, formatNumber } = useLocale();
  const view = useExplorationView();
  const initialSearch = initialQuery ?? query;
  const [search, setSearch] = useState(initialSearch);
  const [appliedSearch, setAppliedSearch] = useState(initialSearch);
  const [filters, setFilters] = useState(() => initialFilters(view.search));
  const [sources, setSources] = useState<readonly SourceSummary[]>([]);
  const [technical, setTechnical] = useState(() => {
    try {
      return localStorage.getItem('stackplore.resources.technical') !== 'hidden';
    } catch {
      return true;
    }
  });
  useEffect(() => {
    setSearch(initialSearch);
    setAppliedSearch(initialSearch);
  }, [initialSearch]);
  useEffect(() => {
    setFilters(initialFilters(view.search));
  }, [view.search]);
  useEffect(() => {
    const controller = new AbortController();
    void api.listSources(controller.signal).then(
      (items) => {
        if (!controller.signal.aborted) setSources(items);
      },
      () => {
        if (!controller.signal.aborted) setSources([]);
      },
    );
    return () => {
      controller.abort();
    };
  }, [api]);
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductResourcePage> =>
      api.listResources({
        ...Object.fromEntries(Object.entries(filters).filter(([, value]) => value !== '')),
        ...(appliedSearch === '' ? {} : { search: appliedSearch }),
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(cursor === undefined ? {} : { cursor }),
        limit: 100,
        signal,
      }),
    [api, appliedSearch, filters, selectedSource],
  );
  const page = useCursorPage(load);
  const change = <Key extends keyof Filters>(key: Key, parameter: string, value: Filters[Key]) => {
    setFilters((current) => ({ ...current, [key]: value }));
    view.write(parameter, value ?? '');
  };
  const submit = (event: SyntheticEvent<HTMLFormElement, SubmitEvent>) => {
    event.preventDefault();
    const value = search.trim();
    setAppliedSearch(value);
    selectQuery(value);
  };
  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t(searchMode ? 'route.search.title' : 'route.resources.title')}</h1>
        <p>{t(searchMode ? 'route.search.description' : 'route.resources.description')}</p>
      </header>
      <form className="resource-search" onSubmit={submit} role="search">
        <label htmlFor="resource-source">{t('product.source.label')}</label>
        <select
          id="resource-source"
          onChange={(event) => {
            selectSource(event.target.value);
          }}
          value={selectedSource ?? sources[0]?.id ?? ''}
        >
          {sources.map((source) => (
            <option key={source.id} value={source.id}>
              {source.name}
            </option>
          ))}
        </select>
        <label htmlFor="resource-search">{t('product.search.label')}</label>
        <div>
          <input
            id="resource-search"
            maxLength={200}
            onChange={(event) => {
              setSearch(event.target.value);
            }}
            placeholder={t('product.search.placeholder')}
            value={search}
          />
          <Button type="submit">{t('product.search.submit')}</Button>
        </div>
      </form>
      <div className="exploration-filters">
        <label>
          {t('explore.filter.provider')}
          <select
            value={filters.provider ?? ''}
            onChange={(event) => {
              change(
                'provider',
                'provider',
                event.target.value === ''
                  ? undefined
                  : (event.target.value as (typeof providerValues)[number]),
              );
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {providerValues.map((value) => (
              <option key={value}>{value}</option>
            ))}
          </select>
        </label>
        <label>
          {t('explore.filter.health')}
          <select
            value={filters.health ?? ''}
            onChange={(event) => {
              change(
                'health',
                'health',
                event.target.value === ''
                  ? undefined
                  : (event.target.value as (typeof healthValues)[number]),
              );
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {healthValues.map((value) => (
              <option key={value} value={value}>
                {t(`product.health.${value}`)}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t('explore.filter.availability')}
          <select
            value={filters.availability ?? ''}
            onChange={(event) => {
              change(
                'availability',
                'availability',
                event.target.value === ''
                  ? undefined
                  : (event.target.value as (typeof availabilityValues)[number]),
              );
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {availabilityValues.map((value) => (
              <option key={value} value={value}>
                {availabilityLabel(value, t)}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t('explore.filter.environment')}
          <input
            dir="ltr"
            maxLength={128}
            key={`environment-${filters.environmentId ?? ''}`}
            defaultValue={filters.environmentId ?? ''}
            onKeyDown={(event) => {
              if (event.key === 'Enter') event.currentTarget.blur();
            }}
            onBlur={(event) => {
              change('environmentId', 'environment_id', event.target.value);
            }}
          />
        </label>
        <label>
          {t('explore.filter.kind')}
          <input
            dir="ltr"
            maxLength={96}
            key={`kind-${filters.kind ?? ''}`}
            defaultValue={filters.kind ?? ''}
            onKeyDown={(event) => {
              if (event.key === 'Enter') event.currentTarget.blur();
            }}
            onBlur={(event) => {
              change('kind', 'kind', event.target.value);
            }}
          />
        </label>
        <label>
          {t('explore.filter.scope')}
          <select
            value={filters.scopeKind ?? ''}
            onChange={(event) => {
              change(
                'scopeKind',
                'scope_kind',
                event.target.value === ''
                  ? undefined
                  : (event.target.value as (typeof scopeValues)[number]),
              );
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {scopeValues.map((value) => (
              <option key={value}>{value}</option>
            ))}
          </select>
        </label>
        <label>
          {t('product.detail.scope')}
          <input
            dir="ltr"
            maxLength={256}
            key={`scope-${filters.scopeName ?? ''}`}
            defaultValue={filters.scopeName ?? ''}
            onKeyDown={(event) => {
              if (event.key === 'Enter') event.currentTarget.blur();
            }}
            onBlur={(event) => {
              change('scopeName', 'scope_name', event.target.value);
            }}
          />
        </label>
        <label>
          {t('explore.filter.sort')}
          <select
            value={filters.sort}
            onChange={(event) => {
              change('sort', 'sort', event.target.value as Filters['sort']);
            }}
          >
            <option value="name">{t('explore.filter.name')}</option>
            <option value="kind">{t('explore.filter.type')}</option>
            <option value="observed_at">{t('explore.filter.observed')}</option>
          </select>
        </label>
        <label>
          {t('explore.filter.order')}
          <select
            value={filters.direction}
            onChange={(event) => {
              change('direction', 'direction', event.target.value === 'desc' ? 'desc' : 'asc');
            }}
          >
            <option value="asc">{t('explore.filter.ascending')}</option>
            <option value="desc">{t('explore.filter.descending')}</option>
          </select>
        </label>
      </div>
      <details className="exploration-columns">
        <summary>{t('explore.filter.columns')}</summary>
        <label>
          <input
            type="checkbox"
            checked={technical}
            onChange={(event) => {
              setTechnical(event.target.checked);
              try {
                localStorage.setItem(
                  'stackplore.resources.technical',
                  event.target.checked ? 'visible' : 'hidden',
                );
              } catch {
                /* Preferences may be unavailable in a private session. */
              }
            }}
          />
          {t('explore.filter.technical')}
        </label>
      </details>
      {page.state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {page.state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {page.state.kind === 'ready' ? (
        <>
          <ObservationNotice data={page.state.data} />
          {!dataAllowed(page.state.data.availability) ? (
            <ProductState text={availabilityLabel(page.state.data.availability, t)} />
          ) : (
            <>
              <p role="status">
                {t('explore.page.loaded')}: {formatNumber(page.state.data.items.length)}
              </p>
              {page.state.data.items.length === 0 ? (
                <ProductState text={t('product.empty')} />
              ) : (
                <div className="resource-list">
                  {page.state.data.items.map((resource) => (
                    <a
                      className="resource-row ui-focus-ring"
                      href={resourceHref(resource)}
                      id={`resource-${resource.id}`}
                      key={resource.id}
                    >
                      <span>
                        <strong>{resource.name}</strong>
                        {technical ? (
                          <small>
                            <TechnicalText>
                              {resource.kind} · {resource.scope_name ?? resource.scope_kind}
                            </TechnicalText>
                          </small>
                        ) : null}
                      </span>
                      <span className={`health-pill health-pill--${resource.health ?? 'unknown'}`}>
                        {t(`product.health.${resource.health ?? 'unknown'}`)}
                      </span>
                      <span className="demo-chip">
                        {resource.synthetic ? t('product.demoBadge') : resource.provider}
                      </span>
                    </a>
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
          )}
        </>
      ) : null}
    </div>
  );
}
