import type { ProductRelationship, ProductResource } from '@stackplore/api-client';

export interface TopologyPoint {
  readonly x: number;
  readonly y: number;
}
export type TopologyGrouping = 'scope' | 'environment' | 'kind' | 'none';
export type TopologyNeighborhood = 'all' | 'neighbors' | 'upstream' | 'downstream';
export const topologyNodeWidth = 176;
export const topologyNodeHeight = 90;
export const maximumGraphNodes = 200;
export const topologyListPageSize = 50;
export const topologyViewportHeight = 560;

export function topologyResourceHref(resource: ProductResource) {
  return `/resources/${encodeURIComponent(resource.source_id)}/${encodeURIComponent(resource.provider)}/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.id)}?source=${encodeURIComponent(resource.source_id)}`;
}

export function topologyGroup(resource: ProductResource, grouping: TopologyGrouping) {
  if (grouping === 'scope') return `${resource.scope_kind}: ${resource.scope_name ?? '—'}`;
  if (grouping === 'environment') return resource.environment_id;
  if (grouping === 'kind') return resource.kind;
  return '';
}

export function layoutTopology(resources: readonly ProductResource[], grouping: TopologyGrouping) {
  const ordered = [...resources].sort(
    (left, right) =>
      topologyGroup(left, grouping).localeCompare(topologyGroup(right, grouping)) ||
      left.name.localeCompare(right.name) ||
      left.id.localeCompare(right.id),
  );
  const points = new Map<string, TopologyPoint>();
  const groups: { readonly label: string; readonly y: number; readonly height: number }[] = [];
  let y = 16;
  for (const label of [...new Set(ordered.map((resource) => topologyGroup(resource, grouping)))]) {
    const members = ordered.filter((resource) => topologyGroup(resource, grouping) === label);
    const height = Math.ceil(members.length / 4) * 118 + (label === '' ? 16 : 42);
    groups.push({ label, y, height });
    members.forEach((resource, index) => {
      points.set(resource.id, {
        x: 120 + (index % 4) * 230,
        y: y + (label === '' ? 60 : 86) + Math.floor(index / 4) * 118,
      });
    });
    y += height + 18;
  }
  return { points, groups, width: 940, height: Math.max(360, y) };
}

export function routeTopologyEdge(
  from: TopologyPoint,
  to: TopologyPoint,
  obstacles: readonly TopologyPoint[],
  lane = 0,
): readonly TopologyPoint[] {
  const slot = Math.abs(lane) % 5;
  const offset = (Math.floor(Math.abs(lane) / 5) % 200) / 200;
  const horizontalSide = to.x < from.x ? -1 : 1;
  const verticalSide = to.y > from.y ? -1 : 1;
  if (from.x !== to.x && from.y === to.y) {
    const direct = [
      { x: from.x + (horizontalSide * topologyNodeWidth) / 2, y: from.y + slot * 6 + offset },
      { x: to.x - (horizontalSide * topologyNodeWidth) / 2, y: to.y + slot * 6 + offset },
    ];
    if (
      !obstacles.some((obstacle) =>
        segmentCrossesCard(direct[0] ?? from, direct[1] ?? to, obstacle),
      )
    )
      return direct;
  }
  if (from.y !== to.y && from.x === to.x) {
    const direct = [
      { x: from.x + slot * 6 + offset, y: from.y - (verticalSide * topologyNodeHeight) / 2 },
      { x: to.x + slot * 6 + offset, y: to.y + (verticalSide * topologyNodeHeight) / 2 },
    ];
    if (
      !obstacles.some((obstacle) =>
        segmentCrossesCard(direct[0] ?? from, direct[1] ?? to, obstacle),
      )
    )
      return direct;
  }
  // Grid gutters stay outside every card, including cards between the endpoints.
  for (const side of [horizontalSide, -horizontalSide]) {
    for (const entrySide of [verticalSide, -verticalSide]) {
      const departure = {
        x: from.x + (side * topologyNodeWidth) / 2,
        y: from.y + slot * 6 - 12 + offset,
      };
      const channelX = from.x + side * (topologyNodeWidth / 2 + 14 + slot * 4 + offset);
      const arrival = {
        x: to.x + slot * 8 - 16 + offset,
        y: to.y + (entrySide * topologyNodeHeight) / 2,
      };
      const channelY = arrival.y + entrySide * (8 + slot * 3 + offset);
      const route = [
        departure,
        { x: channelX, y: departure.y },
        { x: channelX, y: channelY },
        { x: arrival.x, y: channelY },
        arrival,
      ];
      if (
        route.every(
          (point, index) =>
            index === 0 ||
            !obstacles.some((obstacle) =>
              segmentCrossesCard(route[index - 1] ?? point, point, obstacle),
            ),
        )
      )
        return route;
    }
  }
  return [];
}

function segmentCrossesCard(from: TopologyPoint, to: TopologyPoint, card: TopologyPoint) {
  const left = card.x - topologyNodeWidth / 2;
  const right = card.x + topologyNodeWidth / 2;
  const top = card.y - topologyNodeHeight / 2;
  const bottom = card.y + topologyNodeHeight / 2;
  if (from.x === to.x)
    return (
      from.x > left &&
      from.x < right &&
      Math.min(from.y, to.y) < bottom &&
      Math.max(from.y, to.y) > top
    );
  return (
    from.y > top &&
    from.y < bottom &&
    Math.min(from.x, to.x) < right &&
    Math.max(from.x, to.x) > left
  );
}

export function topologyEdgePath(
  from: TopologyPoint,
  to: TopologyPoint,
  obstacles: readonly TopologyPoint[] = [from, to],
  lane = 0,
) {
  return routeTopologyEdge(from, to, obstacles, lane)
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${String(point.x)} ${String(point.y)}`)
    .join(' ');
}

export function topologyNeighborhood(
  relationships: readonly ProductRelationship[],
  selected: string,
  mode: TopologyNeighborhood,
) {
  if (selected === '' || mode === 'all') return null;
  const result = new Set([selected]);
  for (const edge of relationships) {
    // Co-location has no upstream or downstream meaning.
    if (edge.direction === 'undirected' && mode !== 'neighbors') continue;
    const both = mode === 'neighbors' || edge.direction === 'bidirectional';
    if (edge.from === selected && (both || mode === 'downstream')) result.add(edge.to);
    if (edge.to === selected && (both || mode === 'upstream')) result.add(edge.from);
  }
  return result;
}
