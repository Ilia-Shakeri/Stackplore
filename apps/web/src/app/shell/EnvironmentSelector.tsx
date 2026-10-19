import type { ProductEnvironmentPage } from '@stackplore/api-client';
import { Button, Dialog, DialogContent, DialogTrigger, Select, SelectOption } from '@stackplore/ui';
import { Boxes } from '@stackplore/ui/icons';
import { useNavigate, useRouterState } from '@tanstack/react-router';
import { useCallback, useRef, useState } from 'react';

import { useLocale } from '../../i18n';
import {
  useProductApi,
  useProductSource,
  useProductView,
} from '../../features/product/product-context';
import { useCursorPage } from '../../features/product/use-cursor-page';
import {
  dataAllowed,
  ObservationNotice,
  PageControls,
} from '../../features/product/exploration-ui';
import {
  environmentDestination,
  environmentNavigationSearch,
  readEnvironment,
} from '../../features/product/environment-navigation';

export function EnvironmentSelector() {
  const [open, setOpen] = useState(false);
  const navigated = useRef(false);
  const { t } = useLocale();
  const { selectedSource, query } = useProductSource();
  const view = useProductView();
  const selected = readEnvironment(view.search);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const navigate = useNavigate();
  const choose = (environment?: string, source = selectedSource) => {
    navigated.current = true;
    setOpen(false);
    const destination = environmentDestination(pathname);
    const next = new URLSearchParams(destination === pathname ? view.search : '');
    if (destination === pathname && query !== '') next.set('q', query);
    for (const key of [
      'source',
      'environment_id',
      'topology_environment',
      'topology_selected',
      'resource_id',
      'container',
      'rule_cursor',
    ])
      next.delete(key);
    void navigate({
      to: destination,
      search: {
        ...Object.fromEntries(next),
        ...environmentNavigationSearch(destination, source, environment),
      },
    });
  };
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (value) navigated.current = false;
        setOpen(value);
      }}
    >
      <DialogTrigger asChild>
        <button
          className="app-topbar__notification ui-focus-ring"
          type="button"
          aria-label={`${t('shell.environment.open')}${selected === undefined ? '' : `: ${selected}`}`}
          title={t('shell.environment.open')}
        >
          <Boxes aria-hidden="true" size={18} />
        </button>
      </DialogTrigger>
      <DialogContent
        title={t('shell.environment.title')}
        description={t('shell.environment.description')}
        closeLabel={t('shell.environment.close')}
        onCloseAutoFocus={(event) => {
          if (!navigated.current) return;
          const target = document.getElementById('main-content');
          if (target?.isConnected) {
            event.preventDefault();
            target.focus();
          }
        }}
      >
        {open ? <EnvironmentChoices onChoose={choose} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function EnvironmentChoices({
  onChoose,
}: {
  readonly onChoose: (environment?: string, source?: string) => void;
}) {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { search } = useProductView();
  const selected = readEnvironment(search);
  const { t } = useLocale();
  const [revision, setRevision] = useState(0);
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductEnvironmentPage> =>
      api.listEnvironments({
        signal,
        limit: 100,
        ...(selectedSource === undefined ? {} : { sourceId: selectedSource }),
        ...(cursor === undefined ? {} : { cursor }),
      }),
    [api, selectedSource],
  );
  const page = useCursorPage(load, revision);
  if (page.state.kind === 'loading') return <p role="status">{t('product.loading')}</p>;
  if (page.state.kind === 'error')
    return (
      <div role="alert">
        <p>{t('product.error')}</p>
        <Button
          onClick={() => {
            setRevision((value) => value + 1);
          }}
        >
          {t('shell.environment.reload')}
        </Button>
      </div>
    );
  const data = page.state.data;
  const active = data.items.find(
    (item) =>
      item.id === selected && (selectedSource === undefined || item.source_id === selectedSource),
  );
  const selectedValue =
    selected === undefined
      ? ''
      : active === undefined
        ? 'not-loaded'
        : `${active.source_id}/${active.id}`;
  return (
    <div className="environment-selector__choices">
      <ObservationNotice data={data} />
      {dataAllowed(data.availability) ? (
        <>
          <Select
            aria-label={t('shell.environment.label')}
            value={selectedValue}
            dir="auto"
            onChange={(event) => {
              if (event.currentTarget.value === '') {
                onChoose();
                return;
              }
              const environment = data.items.find(
                (item) => `${item.source_id}/${item.id}` === event.currentTarget.value,
              );
              if (environment !== undefined && dataAllowed(environment.availability))
                onChoose(environment.id, environment.source_id);
            }}
          >
            <SelectOption value="">{t('shell.environment.all')}</SelectOption>
            {selected !== undefined && active === undefined ? (
              <SelectOption value="not-loaded" disabled>
                {t('shell.environment.notLoaded')}: {selected}
              </SelectOption>
            ) : null}
            {data.items.map((environment) => (
              <SelectOption
                key={`${environment.source_id}/${environment.id}`}
                value={`${environment.source_id}/${environment.id}`}
                disabled={!dataAllowed(environment.availability)}
              >
                {environment.name}
              </SelectOption>
            ))}
          </Select>
          {data.items.length === 0 ? <p role="status">{t('shell.environment.empty')}</p> : null}
          <PageControls
            capped={page.capped}
            hasMore={data.page.has_more}
            loading={page.loadingMore}
            failed={page.moreFailed}
            onMore={page.loadMore}
          />
        </>
      ) : (
        <Button
          onClick={() => {
            setRevision((value) => value + 1);
          }}
        >
          {t('shell.environment.reload')}
        </Button>
      )}
    </div>
  );
}
