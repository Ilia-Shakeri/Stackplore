import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import { describe, expect, it } from 'vitest';

import {
  layoutTopology,
  routeTopologyEdge,
  topologyEdgePath,
  topologyNeighborhood,
  topologyNodeHeight,
  topologyNodeWidth,
  type TopologyPoint,
} from './topology-model';

const edge = (
  from: string,
  to: string,
  direction: ProductRelationship['direction'],
): ProductRelationship => ({
  id: `rel1_${'A'.repeat(43)}`,
  from,
  to,
  direction,
  kind: 'depends_on',
  confidence: 1,
  observed_at: '2026-08-16T08:00:00Z',
  synthetic: true,
  evidence: [],
});

describe('topology navigation model', () => {
  it('clips reverse and vertical arrows to their actual target boundary', () => {
    expect(topologyEdgePath({ x: 300, y: 100 }, { x: 100, y: 100 })).toBe('M 212 100 L 188 100');
    expect(topologyEdgePath({ x: 100, y: 100 }, { x: 100, y: 300 })).toBe('M 100 145 L 100 255');
  });
  it('does not infer directed dependency from co-location while honoring both directions', () => {
    const edges = [
      edge('selected', 'down', 'source_to_target'),
      edge('up', 'selected', 'source_to_target'),
      edge('selected', 'shared', 'undirected'),
      edge('selected', 'both', 'bidirectional'),
    ];
    expect(topologyNeighborhood(edges, 'selected', 'upstream')).toEqual(
      new Set(['selected', 'up', 'both']),
    );
    expect(topologyNeighborhood(edges, 'selected', 'downstream')).toEqual(
      new Set(['selected', 'down', 'both']),
    );
    expect(topologyNeighborhood(edges, 'selected', 'neighbors')).toEqual(
      new Set(['selected', 'up', 'down', 'shared', 'both']),
    );
  });

  it('routes long demo links around Checkout and Gateway rather than through either card', () => {
    const api = { x: 120, y: 102 };
    const checkout = { x: 350, y: 102 };
    const gateway = { x: 580, y: 102 };
    const database = { x: 810, y: 102 };
    const redis = { x: 120, y: 220 };
    const worker = { x: 350, y: 220 };
    const cards = [api, checkout, gateway, database, redis, worker];
    for (const [from, to] of [
      [api, database],
      [gateway, api],
      [worker, database],
      [api, redis],
    ] as const) {
      const route = routeTopologyEdge(from, to, cards);
      expect(route.length).toBeGreaterThan(1);
      expect(onBoundary(required(route.at(0)), from)).toBe(true);
      expect(onBoundary(required(route.at(-1)), to)).toBe(true);
      for (let index = 1; index < route.length; index += 1) {
        for (const card of cards)
          expect(segmentHitsCard(required(route[index - 1]), required(route[index]), card)).toBe(
            false,
          );
      }
    }
    expect(routeTopologyEdge(api, database, cards).length).toBeGreaterThan(2);
    expect(routeTopologyEdge(gateway, api, cards).length).toBeGreaterThan(2);
  });

  it('bounds 1000 distinct routes among 200 grouped cards and never enters a card', () => {
    const resources = Array.from({ length: 200 }, (_, index): ProductResource => ({
      id: `r1_${String(index).padStart(43, 'A')}`,
      source_id: 'source_demo',
      provider: 'demo',
      environment_id: `env_${String(index % 3)}`,
      kind: 'demo.service',
      scope_kind: 'project',
      scope_name: `project-${String(index % 4)}`,
      name: `Node ${String(index)}`,
      synthetic: true,
      observed_at: '2026-08-16T08:00:00Z',
    }));
    const layout = layoutTopology(resources, 'scope');
    const cards = [...layout.points.values()];
    const paths = new Set<string>();
    for (let lane = 0; lane < 1000; lane += 1) {
      const from = required(cards[lane % cards.length]);
      const to = required(cards[(lane * 17 + 31) % cards.length]);
      const route = routeTopologyEdge(from, to, cards, lane);
      expect(route.length).toBeGreaterThan(1);
      paths.add(topologyEdgePath(from, to, cards, lane));
      for (const point of route) {
        expect(point.x).toBeGreaterThanOrEqual(0);
        expect(point.x).toBeLessThanOrEqual(layout.width);
        expect(point.y).toBeGreaterThanOrEqual(0);
        expect(point.y).toBeLessThanOrEqual(layout.height);
      }
      for (let index = 1; index < route.length; index += 1) {
        expect(
          cards.some((card) =>
            segmentHitsCard(required(route[index - 1]), required(route[index]), card),
          ),
        ).toBe(false);
      }
    }
    expect(paths.size).toBe(1000);
  });

  it('keeps self loops outside their own card', () => {
    const card = { x: 120, y: 102 };
    const route = routeTopologyEdge(card, card, [card], 999);
    expect(route.length).toBeGreaterThan(2);
    for (let index = 1; index < route.length; index += 1)
      expect(segmentHitsCard(required(route[index - 1]), required(route[index]), card)).toBe(false);
  });
});

function required<T>(value: T | undefined): T {
  if (value === undefined) throw new Error('Fixture missing.');
  return value;
}
function onBoundary(point: TopologyPoint, card: TopologyPoint) {
  const dx = Math.abs(point.x - card.x);
  const dy = Math.abs(point.y - card.y);
  return (
    dx <= topologyNodeWidth / 2 &&
    dy <= topologyNodeHeight / 2 &&
    (dx === topologyNodeWidth / 2 || dy === topologyNodeHeight / 2)
  );
}
function segmentHitsCard(from: TopologyPoint, to: TopologyPoint, card: TopologyPoint) {
  let enter = 0;
  let leave = 1;
  for (const [axis, half] of [
    ['x', topologyNodeWidth / 2],
    ['y', topologyNodeHeight / 2],
  ] as const) {
    const low = card[axis] - half + 0.00001;
    const high = card[axis] + half - 0.00001;
    const delta = to[axis] - from[axis];
    if (delta === 0) {
      if (from[axis] < low || from[axis] > high) return false;
    } else {
      const first = (low - from[axis]) / delta;
      const last = (high - from[axis]) / delta;
      enter = Math.max(enter, Math.min(first, last));
      leave = Math.min(leave, Math.max(first, last));
      if (enter > leave) return false;
    }
  }
  return enter <= leave;
}
