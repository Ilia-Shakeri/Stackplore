import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import {
  useId,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type WheelEvent as ReactWheelEvent,
} from 'react';

import { useLocale } from '../../i18n';

interface TopologyGraphProps {
  readonly relationships: readonly ProductRelationship[];
  readonly resources: readonly ProductResource[];
}

interface Point {
  readonly x: number;
  readonly y: number;
}

const graphWidth = 960;
const graphHeight = 560;
const nodeWidth = 168;
const nodeHeight = 58;

function readGraphParameter(name: string) {
  if (typeof window === 'undefined') return '';
  return new URLSearchParams(window.location.search).get(name)?.slice(0, 200) ?? '';
}

function writeGraphParameter(name: string, value: string) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (value === '') url.searchParams.delete(name);
  else url.searchParams.set(name, value);
  window.history.replaceState(window.history.state, '', url);
}

export function TopologyGraph({ relationships, resources }: TopologyGraphProps) {
  const { t } = useLocale();
  const markerId = useId().replaceAll(':', '');
  const frame = useRef<HTMLDivElement>(null);
  const drag = useRef<{
    readonly id: number;
    readonly startX: number;
    readonly startY: number;
    readonly x: number;
    readonly y: number;
  } | null>(null);
  const [kind, setKind] = useState(() => readGraphParameter('topology_kind'));
  const [query, setQuery] = useState(() => readGraphParameter('topology_q'));
  const [scale, setScale] = useState(1);
  const [pan, setPan] = useState<Point>({ x: 0, y: 0 });
  const kinds = useMemo(
    () => [...new Set(resources.map((resource) => resource.kind))].sort(),
    [resources],
  );
  const visible = useMemo(() => {
    const term = query.trim().toLocaleLowerCase();
    return resources.filter(
      (resource) =>
        (kind === '' || resource.kind === kind) &&
        (term === '' ||
          resource.name.toLocaleLowerCase().includes(term) ||
          resource.kind.toLocaleLowerCase().includes(term) ||
          resource.id.toLocaleLowerCase().includes(term)),
    );
  }, [kind, query, resources]);
  const visibleIds = useMemo(() => new Set(visible.map((resource) => resource.id)), [visible]);
  const edges = useMemo(
    () =>
      relationships.filter(
        (relationship) => visibleIds.has(relationship.from) && visibleIds.has(relationship.to),
      ),
    [relationships, visibleIds],
  );
  const points = useMemo(() => layoutNodes(visible), [visible]);

  const zoom = (amount: number) => {
    setScale((value) => Math.min(2.5, Math.max(0.45, value + amount)));
  };
  const fit = () => {
    setScale(1);
    setPan({ x: 0, y: 0 });
  };
  const onWheel = (event: ReactWheelEvent<SVGSVGElement>) => {
    event.preventDefault();
    zoom(event.deltaY > 0 ? -0.1 : 0.1);
  };
  const onPointerDown = (event: ReactPointerEvent<SVGSVGElement>) => {
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
    setPan({
      x: current.x + (event.clientX - current.startX) / scale,
      y: current.y + (event.clientY - current.startY) / scale,
    });
  };
  const endDrag = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (drag.current?.id === event.pointerId) drag.current = null;
  };
  const fullscreen = () => {
    if (frame.current?.requestFullscreen !== undefined) {
      void frame.current.requestFullscreen().catch(() => undefined);
    }
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
              setQuery(event.target.value);
              writeGraphParameter('topology_q', event.target.value);
            }}
            placeholder={t('product.topology.searchPlaceholder')}
            type="search"
            value={query}
          />
        </label>
        <label>
          <span>{t('product.topology.kindLabel')}</span>
          <select
            onChange={(event) => {
              setKind(event.target.value);
              writeGraphParameter('topology_kind', event.target.value);
            }}
            value={kind}
          >
            <option value="">{t('product.topology.allKinds')}</option>
            {kinds.map((value) => (
              <option key={value} value={value}>
                {value}
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
        </div>
      </div>
      {visible.length === 0 ? (
        <p className="topology-empty" role="status">
          {t('product.topology.empty')}
        </p>
      ) : (
        <div className="topology-canvas">
          <svg
            aria-label={t('product.topology.graphLabel')}
            onPointerCancel={endDrag}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={endDrag}
            onWheel={onWheel}
            role="img"
            viewBox={['0', '0', graphWidth, graphHeight].join(' ')}
          >
            <defs>
              <marker
                id={markerId}
                markerHeight="8"
                markerWidth="8"
                orient="auto"
                refX="7"
                refY="4"
              >
                <path d="M0,0 L8,4 L0,8 z" />
              </marker>
            </defs>
            <g transform={`translate(${String(pan.x)} ${String(pan.y)}) scale(${String(scale)})`}>
              {edges.map((edge) => {
                const from = points.get(edge.from);
                const to = points.get(edge.to);
                if (from === undefined || to === undefined) return null;
                return (
                  <line
                    key={edge.id}
                    markerEnd={`url(#${markerId})`}
                    x1={from.x + nodeWidth / 2}
                    x2={to.x - nodeWidth / 2}
                    y1={from.y}
                    y2={to.y}
                  >
                    <title>{`${edge.kind}, ${String(Math.round(edge.confidence * 100))}%`}</title>
                  </line>
                );
              })}
              {visible.map((resource) => {
                const point = points.get(resource.id);
                if (point === undefined) return null;
                return (
                  <a
                    href={`/resources/${encodeURIComponent(resource.source_id)}/${encodeURIComponent(resource.provider)}/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.id)}?source=${encodeURIComponent(resource.source_id)}`}
                    key={resource.id}
                  >
                    <g className={`topology-node topology-node--${resource.health ?? 'unknown'}`}>
                      <rect
                        height={nodeHeight}
                        rx="10"
                        width={nodeWidth}
                        x={point.x - nodeWidth / 2}
                        y={point.y - nodeHeight / 2}
                      />
                      <text textAnchor="middle" x={point.x} y={point.y - 4}>
                        {shortLabel(resource.name)}
                      </text>
                      <text
                        className="topology-node__kind"
                        textAnchor="middle"
                        x={point.x}
                        y={point.y + 16}
                      >
                        {shortLabel(resource.kind)}
                      </text>
                    </g>
                  </a>
                );
              })}
            </g>
          </svg>
          <svg
            aria-label={t('product.topology.minimapLabel')}
            className="topology-minimap"
            role="img"
            viewBox={['0', '0', graphWidth, graphHeight].join(' ')}
          >
            {visible.map((resource) => {
              const point = points.get(resource.id);
              return point === undefined ? null : (
                <circle cx={point.x} cy={point.y} key={resource.id} r="12" />
              );
            })}
          </svg>
        </div>
      )}
    </section>
  );
}

function layoutNodes(resources: readonly ProductResource[]) {
  const ordered = [...resources].sort((left, right) => {
    const leftGroup = `${left.scope_name ?? ''}\u0000${left.kind}\u0000${left.name}`;
    const rightGroup = `${right.scope_name ?? ''}\u0000${right.kind}\u0000${right.name}`;
    return leftGroup.localeCompare(rightGroup);
  });
  const columns = Math.max(1, Math.ceil(Math.sqrt(ordered.length * 1.6)));
  const columnGap = (graphWidth - 120) / Math.max(1, columns - 1);
  const rows = Math.ceil(ordered.length / columns);
  const rowGap = (graphHeight - 100) / Math.max(1, rows - 1);
  return new Map(
    ordered.map((resource, index) => [
      resource.id,
      {
        x: 60 + (index % columns) * columnGap,
        y: 50 + Math.floor(index / columns) * rowGap,
      },
    ]),
  );
}

function shortLabel(value: string) {
  return value.length > 22 ? `${value.slice(0, 21)}…` : value;
}
