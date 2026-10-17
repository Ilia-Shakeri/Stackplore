import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import { useId, useMemo, type PointerEvent as ReactPointerEvent } from 'react';
import { useLocale } from '../../i18n';
import { evidence, relationshipKeys } from './topology-presentation';
import {
  topologyEdgePath,
  topologyNodeHeight,
  topologyNodeWidth,
  topologyResourceHref,
  topologyViewportHeight,
  type TopologyPoint,
  type layoutTopology,
} from './topology-model';

export function TopologyCanvas({
  graphResources,
  edges,
  layout,
  highlights,
  selected,
  scale,
  pan,
  onPointerDown,
  onPointerMove,
  endDrag,
  selectNode,
}: {
  readonly graphResources: readonly ProductResource[];
  readonly edges: readonly ProductRelationship[];
  readonly layout: ReturnType<typeof layoutTopology>;
  readonly highlights: ReadonlySet<string> | null;
  readonly selected: string;
  readonly scale: number;
  readonly pan: TopologyPoint;
  readonly onPointerDown: (event: ReactPointerEvent<SVGSVGElement>) => void;
  readonly onPointerMove: (event: ReactPointerEvent<SVGSVGElement>) => void;
  readonly endDrag: (event: ReactPointerEvent<SVGSVGElement>) => void;
  readonly selectNode: (value: string) => void;
}) {
  const { t } = useLocale();
  const markerId = useId().replaceAll(':', '');
  const names = useMemo(
    () => new Map(graphResources.map((resource) => [resource.id, resource])),
    [graphResources],
  );
  const geometry = useMemo(() => {
    const obstacles = [...layout.points.values()];
    return edges.flatMap((edge, index) => {
      const from = layout.points.get(edge.from);
      const to = layout.points.get(edge.to);
      if (from === undefined || to === undefined) return [];
      const path = topologyEdgePath(from, to, obstacles, index);
      return path === '' ? [] : [{ edge, path }];
    });
  }, [edges, layout]);
  const direction = (edge: ProductRelationship) => t(`topology.direction.${edge.direction}`);
  return (
    <div className={`topology-canvas${highlights === null ? '' : ' topology-highlight-active'}`}>
      <svg
        aria-label={t('product.topology.graphLabel')}
        onPointerCancel={endDrag}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        role="group"
        viewBox={`0 0 ${String(layout.width)} ${String(topologyViewportHeight)}`}
      >
        <defs>
          <marker
            id={markerId}
            markerHeight="10"
            markerWidth="10"
            markerUnits="userSpaceOnUse"
            orient="auto-start-reverse"
            refX="10"
            refY="5"
          >
            <path d="M0,0 L10,5 L0,10 z" />
          </marker>
        </defs>
        <g transform={`translate(${String(pan.x)} ${String(pan.y)}) scale(${String(scale)})`}>
          {layout.groups
            .filter((group) => group.label !== '')
            .map((group) => (
              <g className="topology-group" key={group.label}>
                <rect x="12" y={group.y} width={layout.width - 24} height={group.height} rx="12" />
                <text x="28" y={group.y + 24} data-ui-technical direction="ltr">
                  {group.label}
                </text>
              </g>
            ))}
          {geometry.map(({ edge, path }) => {
            const inferred = evidence(edge).some((item) => item.certainty === 'inferred');
            return (
              <g key={edge.id}>
                <path
                  aria-hidden="true"
                  d={path}
                  fill="none"
                  stroke="var(--sp-surface)"
                  strokeWidth="6"
                />
                <path
                  aria-hidden="true"
                  className={`topology-edge${inferred ? ' topology-edge--inferred' : ''}${highlights !== null && (!highlights.has(edge.from) || !highlights.has(edge.to)) ? ' topology-muted' : ''}`}
                  d={path}
                  markerEnd={edge.direction === 'undirected' ? undefined : `url(#${markerId})`}
                  markerStart={edge.direction === 'bidirectional' ? `url(#${markerId})` : undefined}
                >
                  <title>{`${names.get(edge.from)?.name ?? edge.from} · ${t(relationshipKeys[edge.kind])} · ${names.get(edge.to)?.name ?? edge.to} · ${direction(edge)}`}</title>
                </path>
              </g>
            );
          })}
          {graphResources.map((resource) => {
            const point = layout.points.get(resource.id);
            if (point === undefined) return null;
            return (
              <a
                aria-label={resource.name}
                href={topologyResourceHref(resource)}
                key={resource.id}
                onFocus={() => {
                  selectNode(resource.id);
                }}
              >
                <g
                  className={`topology-node topology-node--${resource.health ?? 'unknown'}${selected === resource.id ? ' topology-node--selected' : ''}${highlights !== null && !highlights.has(resource.id) ? ' topology-muted' : ''}`}
                >
                  <rect
                    height={topologyNodeHeight}
                    rx="10"
                    width={topologyNodeWidth}
                    x={point.x - topologyNodeWidth / 2}
                    y={point.y - topologyNodeHeight / 2}
                  />
                  <text textAnchor="middle" x={point.x} y={point.y - 19} direction="ltr">
                    {shortLabel(resource.name)}
                  </text>
                  <text
                    className="topology-node__kind"
                    textAnchor="middle"
                    x={point.x}
                    y={point.y + 1}
                    direction="ltr"
                  >
                    {shortLabel(`${resource.provider} · ${resource.kind}`)}
                  </text>
                  <text
                    className="topology-node__kind"
                    textAnchor="middle"
                    x={point.x}
                    y={point.y + 23}
                  >
                    {t(`product.health.${resource.health ?? 'unknown'}`)}
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
        viewBox={`0 0 ${String(layout.width)} ${String(layout.height)}`}
      >
        {graphResources.map((resource) => {
          const point = layout.points.get(resource.id);
          return point === undefined ? null : (
            <circle cx={point.x} cy={point.y} key={resource.id} r="12" />
          );
        })}
      </svg>
    </div>
  );
}
function shortLabel(value: string) {
  const points = Array.from(value);
  return points.length > 24 ? `${points.slice(0, 23).join('')}…` : value;
}
