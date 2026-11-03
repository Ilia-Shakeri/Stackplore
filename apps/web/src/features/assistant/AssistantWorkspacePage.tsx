import type { ProductResource, ProductResourcePage, SourceSummary } from '@stackplore/api-client';
import { Button, Input } from '@stackplore/ui';
import { useEffect, useState, type SyntheticEvent } from 'react';

import { AssistantResourcePanel } from './AssistantResourcePanel';
import { useAuth } from '../auth/auth-context';
import { useProductApi, useProductSource } from '../product/product-context';
import {
  ObservationNotice,
  ProductState,
  availabilityLabel,
  dataAllowed,
  useExplorationView,
} from '../product/exploration-ui';
import type { PageLoad } from '../product/use-cursor-page';
import { WorkspaceNavigation } from '../product/WorkspaceNavigation';
import { useLocale } from '../../i18n';

export function AssistantWorkspacePage() {
  const { t } = useLocale();
  const { snapshot } = useAuth();
  return (
    <div className="route-page assistant-workspace">
      <header className="route-page__header">
        <h1>{t('route.assistant.title')}</h1>
        <p>{t('route.assistant.description')}</p>
      </header>
      <WorkspaceNavigation current="assistant" />
      {snapshot.status === 'authenticated' ? (
        <AssistantSelection key={snapshot.user.id} />
      ) : (
        <p role="status">{t('assistant.workspace.guest')}</p>
      )}
    </div>
  );
}

function AssistantSelection() {
  const api = useProductApi();
  const { t } = useLocale();
  const { selectedSource, selectSource } = useProductSource();
  const view = useExplorationView();
  const resourceId = view.read('resource_id');
  const [sources, setSources] = useState<PageLoad<readonly SourceSummary[]>>({ kind: 'loading' });
  const [revision, setRevision] = useState(0);
  const [query, setQuery] = useState('');
  const [search, setSearch] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    setSources({ kind: 'loading' });
    void api.listSources(controller.signal).then(
      (data) => {
        if (!controller.signal.aborted) setSources({ kind: 'ready', data });
      },
      () => {
        if (!controller.signal.aborted) setSources({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, revision]);
  if (sources.kind === 'loading') return <ProductState text={t('product.loading')} />;
  if (sources.kind === 'error')
    return (
      <ProductState
        text={t('product.error')}
        onRetry={() => {
          setRevision((value) => value + 1);
        }}
      />
    );
  const source =
    selectedSource === undefined
      ? sources.data[0]
      : sources.data.find((item) => item.id === selectedSource);
  const submit = (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSearch(query.trim());
  };
  return (
    <>
      <label className="product-source-select">
        <span>{t('product.source.label')}</span>
        <select
          value={source?.id ?? ''}
          onChange={(event) => {
            selectSource(event.currentTarget.value);
          }}
        >
          <option value="" disabled>
            {t('assistant.workspace.source')}
          </option>
          {sources.data.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
      </label>
      {source === undefined ? (
        <p role="status">{t('product.source.unavailable')}</p>
      ) : !dataAllowed(source.availability) ? (
        <p role="status">{availabilityLabel(source.availability, t)}</p>
      ) : resourceId !== '' ? (
        <>
          <Button
            variant="outline"
            onClick={() => {
              view.write('resource_id', '');
            }}
          >
            {t('assistant.workspace.change')}
          </Button>
          <SelectedAssistantResource
            key={`${source.id}:${resourceId}`}
            source={source.id}
            id={resourceId}
          />
        </>
      ) : (
        <section aria-labelledby="assistant-resource-choice">
          <h2 id="assistant-resource-choice">{t('assistant.workspace.choose')}</h2>
          <p>{t('assistant.workspace.scope')}</p>
          <form className="exploration-filters" onSubmit={submit}>
            <label>
              <span>{t('assistant.workspace.search')}</span>
              <Input
                value={query}
                maxLength={200}
                onChange={(event) => {
                  setQuery(event.currentTarget.value);
                }}
              />
            </label>
            <Button type="submit">{t('assistant.workspace.find')}</Button>
          </form>
          <ResourceChoices key={`${source.id}:${search}`} source={source.id} search={search} />
        </section>
      )}
    </>
  );
}

function SelectedAssistantResource({
  source,
  id,
}: {
  readonly source: string;
  readonly id: string;
}) {
  const api = useProductApi();
  const { t } = useLocale();
  const [state, setState] = useState<PageLoad<ProductResource>>({ kind: 'loading' });
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void api.getResource(id, controller.signal, source).then(
      (data) => {
        if (controller.signal.aborted) return;
        setState(
          data.id === id && data.source_id === source ? { kind: 'ready', data } : { kind: 'error' },
        );
      },
      () => {
        if (!controller.signal.aborted) setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, source, id, revision]);
  if (state.kind === 'loading') return <ProductState text={t('product.loading')} />;
  if (state.kind === 'error')
    return (
      <ProductState
        text={t('product.error')}
        onRetry={() => {
          setRevision((value) => value + 1);
        }}
      />
    );
  return (
    <section aria-labelledby="assistant-selected-resource">
      <h2 id="assistant-selected-resource">{state.data.name}</h2>
      <p>{t('assistant.workspace.scope')}</p>
      <AssistantResourcePanel resource={state.data} defaultOpen />
    </section>
  );
}

function ResourceChoices({ source, search }: { readonly source: string; readonly search: string }) {
  const api = useProductApi();
  const view = useExplorationView();
  const { t, formatNumber } = useLocale();
  const [cursors, setCursors] = useState<readonly (string | undefined)[]>([undefined]);
  const [index, setIndex] = useState(0);
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<PageLoad<ProductResourcePage>>({ kind: 'loading' });
  const cursor = cursors[index];
  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void api
      .listResources({
        sourceId: source,
        search,
        limit: 10,
        signal: controller.signal,
        ...(cursor === undefined ? {} : { cursor }),
      })
      .then(
        (data) => {
          if (!controller.signal.aborted) setState({ kind: 'ready', data });
        },
        () => {
          if (!controller.signal.aborted) setState({ kind: 'error' });
        },
      );
    return () => {
      controller.abort();
    };
  }, [api, source, search, cursor, revision]);
  return (
    <>
      {state.kind === 'loading' ? (
        <ProductState text={t('product.loading')} />
      ) : state.kind === 'error' ? (
        <ProductState
          text={t('product.error')}
          onRetry={() => {
            setRevision((value) => value + 1);
          }}
        />
      ) : (
        <>
          <ObservationNotice data={state.data} />
          {dataAllowed(state.data.availability) ? (
            <ul className="assistant-resource-choices">
              {state.data.items
                .slice(0, 10)
                .filter((item) => item.source_id === source)
                .map((item) => (
                  <li key={item.id}>
                    <Button
                      variant="outline"
                      onClick={() => {
                        view.write('resource_id', item.id);
                      }}
                    >
                      {item.name}
                    </Button>
                  </li>
                ))}
            </ul>
          ) : null}
          {dataAllowed(state.data.availability) && state.data.items.length === 0 ? (
            <p>{t('insight.empty')}</p>
          ) : null}
          {index === 9 && state.data.page.has_more ? (
            <p role="status">{t('assistant.workspace.refine')}</p>
          ) : null}
        </>
      )}
      <nav className="exploration-pagination" aria-label={t('assistant.workspace.pages')}>
        <Button
          variant="outline"
          disabled={index === 0 || state.kind === 'loading'}
          onClick={() => {
            setIndex((value) => value - 1);
          }}
        >
          {t('assistant.workspace.previous')}
        </Button>
        <span>{formatNumber(index + 1)}</span>
        <Button
          variant="outline"
          disabled={
            state.kind !== 'ready' ||
            !dataAllowed(state.data.availability) ||
            !state.data.page.has_more ||
            index >= 9
          }
          onClick={() => {
            if (state.kind !== 'ready' || !state.data.page.has_more) return;
            const next = state.data.page.next_cursor;
            if (cursors.slice(0, index + 1).includes(next)) {
              setState({ kind: 'error' });
              return;
            }
            setCursors([...cursors.slice(0, index + 1), next]);
            setIndex((value) => value + 1);
          }}
        >
          {t('assistant.workspace.next')}
        </Button>
      </nav>
    </>
  );
}
