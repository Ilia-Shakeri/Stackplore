import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from 'react';

import {
  layoutTopology,
  maximumGraphNodes,
  topologyNeighborhood,
  topologyResourceHref,
  topologyViewportHeight,
  type TopologyGrouping,
  type TopologyNeighborhood,
  type TopologyPoint,
} from './topology-model';
import { useLocale } from '../../i18n';
import { TopologyList } from './TopologyList';
import { TopologyCanvas } from './TopologyCanvas';
import './topology.css';
import { useExplorationView } from './exploration-ui';

interface TopologyGraphProps {
  readonly relationships: readonly ProductRelationship[];
  readonly resources: readonly ProductResource[];
}
const groupingValues = ['scope', 'environment', 'kind', 'none'] as const;
const neighborhoodValues = ['all', 'neighbors', 'upstream', 'downstream'] as const;
function readGrouping(value: string): TopologyGrouping {
  return groupingValues.find((candidate) => candidate === value) ?? 'scope';
}
function readNeighborhood(value: string): TopologyNeighborhood {
  return neighborhoodValues.find((candidate) => candidate === value) ?? 'all';
}
export function TopologyGraph({ relationships, resources }: TopologyGraphProps) {
  const { t } = useLocale();
  const view = useExplorationView();
  const parameter = (name: string) => view.read(`topology_${name}`, name === 'scope' ? 300 : 200);
  const writeParameter = (name: string, value: string) => {
    view.write(`topology_${name}`, value);
  };
  const frame = useRef<HTMLElement>(null);
  const drag = useRef<{
    readonly id: number;
    readonly startX: number;
    readonly startY: number;
    readonly x: number;
    readonly y: number;
  } | null>(null);
  const [kind, setKind] = useState(() => parameter('kind'));
  const [query, setQuery] = useState(() => parameter('q'));
  const [environment, setEnvironment] = useState(() => parameter('environment'));
  const [scope, setScope] = useState(() => parameter('scope'));
  const [health, setHealth] = useState(() => parameter('health'));
  const [grouping, setGrouping] = useState<TopologyGrouping>(() =>
    readGrouping(parameter('group')),
  );
  const [selected, setSelected] = useState(() => parameter('selected'));
  const [neighborhood, setNeighborhood] = useState<TopologyNeighborhood>(() =>
    readNeighborhood(parameter('neighbors')),
  );
  const [scale, setScale] = useState(1);
  const [pan, setPan] = useState<TopologyPoint>({ x: 0, y: 0 });
  useEffect(() => {
    const restore = () => {
      setKind(parameter('kind'));
      setQuery(parameter('q'));
      setEnvironment(parameter('environment'));
      setScope(parameter('scope'));
      setHealth(parameter('health'));
      setGrouping(readGrouping(parameter('group')));
      setSelected(parameter('selected'));
      setNeighborhood(readNeighborhood(parameter('neighbors')));
    };
    restore();
  }, [view.search]);
  const kinds = useMemo(() => unique(resources.map((resource) => resource.kind)), [resources]);
  const environments = useMemo(
    () => unique(resources.map((resource) => resource.environment_id)),
    [resources],
  );
  const scopes = useMemo(
    () =>
      unique(resources.map((resource) => `${resource.scope_kind}: ${resource.scope_name ?? '—'}`)),
    [resources],
  );
  const visible = useMemo(() => {
    const term = query.trim().toLocaleLowerCase();
    return resources.filter(
      (resource) =>
        (kind === '' || resource.kind === kind) &&
        (environment === '' || resource.environment_id === environment) &&
        (scope === '' || `${resource.scope_kind}: ${resource.scope_name ?? '—'}` === scope) &&
        (health === '' || (resource.health ?? 'unknown') === health) &&
        (term === '' ||
          `${resource.name}\n${resource.kind}\n${resource.id}`.toLocaleLowerCase().includes(term)),
    );
  }, [environment, health, kind, query, resources, scope]);
  const visibleIds = useMemo(() => new Set(visible.map((resource) => resource.id)), [visible]);
  const edges = useMemo(
    () => relationships.filter((edge) => visibleIds.has(edge.from) && visibleIds.has(edge.to)),
    [relationships, visibleIds],
  );
  const graphResources = useMemo(() => visible.slice(0, maximumGraphNodes), [visible]);
  const layout = useMemo(
    () => layoutTopology(graphResources, grouping),
    [graphResources, grouping],
  );
  const names = useMemo(
    () => new Map(resources.map((resource) => [resource.id, resource])),
    [resources],
  );
  const highlights = useMemo(
    () => topologyNeighborhood(edges, visibleIds.has(selected) ? selected : '', neighborhood),
    [edges, neighborhood, selected, visibleIds],
  );
  const selectedResource = visibleIds.has(selected) ? names.get(selected) : undefined;
  const changeFilter = (name: string, value: string, apply: (value: string) => void) => {
    apply(value);
    writeParameter(name, value);
  };
  const selectNode = (value: string) => {
    setSelected(value);
    writeParameter('selected', value);
    const point = layout.points.get(value);
    if (point !== undefined) {
      setPan({
        x: layout.width / 2 - point.x * scale,
        y: topologyViewportHeight / 2 - point.y * scale,
      });
    }
  };
  const zoom = (amount: number) => {
    setScale((value) =>
      Math.min(4, Math.max(Math.min(0.1, topologyViewportHeight / layout.height), value + amount)),
    );
  };
  const fit = () => {
    const nextScale = Math.min(1, topologyViewportHeight / layout.height);
    setScale(nextScale);
    setPan({ x: (layout.width - layout.width * nextScale) / 2, y: 0 });
  };
  const endDrag = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (drag.current?.id === event.pointerId) {
      drag.current = null;
      if (event.currentTarget.hasPointerCapture(event.pointerId))
        event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };
  const onPointerDown = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (
      event.button !== 0 ||
      (event.target instanceof Element && event.target.closest('a') !== null)
    )
      return;
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = {
      id: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      x: pan.x,
      y: pan.y,
    };
  };
  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>) => {
    const current = drag.current;
    if (current?.id !== event.pointerId) return;
    const bounds = event.currentTarget.getBoundingClientRect();
    if (bounds.width === 0 || bounds.height === 0) return;
    const unit = Math.max(layout.width / bounds.width, topologyViewportHeight / bounds.height);
    setPan({
      x: current.x + (event.clientX - current.startX) * unit,
      y: current.y + (event.clientY - current.startY) * unit,
    });
  };
  const fullscreen = () => {
    if (frame.current?.requestFullscreen !== undefined)
      void frame.current.requestFullscreen().catch(() => undefined);
  };

  return (
    <section
      className="topology-workbench"
      aria-label={t('product.topology.graphLabel')}
      ref={frame}
    >
      <div className="topology-tools">
        <label>
          <span>{t('product.topology.searchLabel')}</span>
          <input
            maxLength={200}
            onChange={(event) => {
              changeFilter('q', event.target.value, setQuery);
            }}
            placeholder={t('product.topology.searchPlaceholder')}
            type="search"
            value={query}
          />
        </label>
        <Filter
          label={t('product.topology.kindLabel')}
          allLabel={t('product.topology.allKinds')}
          values={kinds}
          value={kind}
          onChange={(value) => {
            changeFilter('kind', value, setKind);
          }}
        />
        <Filter
          label={t('topology.environment')}
          allLabel={t('topology.allEnvironments')}
          values={environments}
          value={environment}
          onChange={(value) => {
            changeFilter('environment', value, setEnvironment);
          }}
        />
        <Filter
          label={t('topology.scope')}
          allLabel={t('topology.allScopes')}
          values={scopes}
          value={scope}
          onChange={(value) => {
            changeFilter('scope', value, setScope);
          }}
        />
        <Filter
          label={t('topology.health')}
          allLabel={t('topology.allHealth')}
          values={['healthy', 'degraded', 'unhealthy', 'unknown']}
          value={health}
          valueLabel={(value) =>
            t(`product.health.${value as NonNullable<ProductResource['health']>}`)
          }
          onChange={(value) => {
            changeFilter('health', value, setHealth);
          }}
        />
        <label>
          <span>{t('topology.group')}</span>
          <select
            value={grouping}
            onChange={(event) => {
              const value =
                groupingValues.find((candidate) => candidate === event.target.value) ?? 'scope';
              setGrouping(value);
              writeParameter('group', value);
              fit();
            }}
          >
            {groupingValues.map((value) => (
              <option value={value} key={value}>
                {t(`topology.group.${value}`)}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>{t('topology.select')}</span>
          <select
            value={selectedResource === undefined ? '' : selected}
            onChange={(event) => {
              selectNode(event.target.value);
            }}
          >
            <option value="">{t('topology.noSelection')}</option>
            {visible.map((resource) => (
              <option key={resource.id} value={resource.id}>
                {resource.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>{t('topology.highlight')}</span>
          <select
            disabled={selectedResource === undefined}
            value={neighborhood}
            onChange={(event) => {
              const value =
                neighborhoodValues.find((candidate) => candidate === event.target.value) ?? 'all';
              setNeighborhood(value);
              writeParameter('neighbors', value);
            }}
          >
            {neighborhoodValues.map((value) => (
              <option key={value} value={value}>
                {t(`topology.highlight.${value}`)}
              </option>
            ))}
          </select>
        </label>
        <div className="topology-tools__buttons">
          <button
            onClick={() => {
              zoom(0.15);
            }}
            type="button"
          >
            {t('product.topology.zoomIn')}
          </button>
          <button
            onClick={() => {
              zoom(-0.15);
            }}
            type="button"
          >
            {t('product.topology.zoomOut')}
          </button>
          <button onClick={fit} type="button">
            {t('product.topology.fit')}
          </button>
          <button onClick={fullscreen} type="button">
            {t('product.topology.fullscreen')}
          </button>
          {(['Left', 'Right', 'Up', 'Down'] as const).map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => {
                setPan((point) => ({
                  x: point.x + (value === 'Left' ? -80 : value === 'Right' ? 80 : 0),
                  y: point.y + (value === 'Up' ? -80 : value === 'Down' ? 80 : 0),
                }));
              }}
            >
              {t(`topology.pan${value}`)}
            </button>
          ))}
        </div>
      </div>
      <p className="topology-legend">
        <span className="topology-legend__explicit">{t('topology.explicit')}</span>
        <span className="topology-legend__inferred">{t('topology.inferred')}</span>
      </p>
      {visible.length === 0 ? (
        <p className="topology-empty" role="status">
          {t('product.topology.empty')}
        </p>
      ) : (
        <>
          {visible.length > maximumGraphNodes ? (
            <p className="topology-limit" role="status">
              {t('topology.graphLimit', { shown: maximumGraphNodes, count: visible.length })}
            </p>
          ) : null}
          <TopologyCanvas
            graphResources={graphResources}
            edges={edges}
            layout={layout}
            highlights={highlights}
            selected={selected}
            scale={scale}
            pan={pan}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            endDrag={endDrag}
            selectNode={selectNode}
          />
          {selectedResource === undefined ? null : (
            <aside className="topology-selection" aria-label={t('topology.selectedDetail')}>
              <strong>{selectedResource.name}</strong>
              <bdi dir="ltr">
                {selectedResource.provider} · {selectedResource.kind} ·{' '}
                {selectedResource.scope_name ?? selectedResource.scope_kind}
              </bdi>
              <span>{t(`product.health.${selectedResource.health ?? 'unknown'}`)}</span>
              <a href={topologyResourceHref(selectedResource)}>{t('topology.openResource')}</a>
            </aside>
          )}
          <TopologyList
            key={[kind, query, environment, scope, health].join('\u0000')}
            resources={visible}
            relationships={edges}
          />
        </>
      )}
    </section>
  );
}

function Filter({
  label,
  allLabel,
  value,
  values,
  valueLabel,
  onChange,
}: {
  readonly label: string;
  readonly allLabel: string;
  readonly value: string;
  readonly values: readonly string[];
  readonly valueLabel?: (value: string) => string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label>
      <span>{label}</span>
      <select
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        <option value="">{allLabel}</option>
        {values.map((item) => (
          <option value={item} key={item}>
            {valueLabel?.(item) ?? item}
          </option>
        ))}
      </select>
    </label>
  );
}
function unique(values: readonly string[]) {
  return [...new Set(values)].sort();
}
