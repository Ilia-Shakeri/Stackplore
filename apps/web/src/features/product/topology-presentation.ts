import type {
  ProductRelationship,
  ProductResource,
  RelationshipEvidence,
} from '@stackplore/api-client';
import type { TextMessageKey, useLocale } from '../../i18n';
export const relationshipKeys = {
  owns: 'product.topology.owns',
  selects: 'product.topology.selects',
  routes_to: 'product.topology.routesTo',
  depends_on: 'product.topology.dependsOn',
  mounts: 'product.topology.mounts',
  references: 'product.topology.references',
  member_of: 'product.topology.memberOf',
} as const satisfies Record<ProductRelationship['kind'], TextMessageKey>;
export function evidence(edge: ProductRelationship): readonly RelationshipEvidence[] {
  return edge.evidence;
}

export function topologyResourceState(
  resource: ProductResource,
  t: ReturnType<typeof useLocale>['t'],
) {
  const metadata = resource.metadata;
  if (metadata === undefined) return undefined;
  if ('docker' in metadata && resource.kind === 'docker.container') {
    const state = metadata.docker.state;
    return state === 'unknown' ? undefined : t(`metadata.state.${state}`);
  }
  if ('kubernetes' in metadata && metadata.kubernetes.containers.length > 0) {
    const states = new Set(metadata.kubernetes.containers.map((container) => container.state));
    if (states.size > 1) return t('topology.mixedState');
    const state = metadata.kubernetes.containers[0]?.state;
    return state === undefined || state === 'unknown' ? undefined : t(`metadata.state.${state}`);
  }
  return undefined;
}
