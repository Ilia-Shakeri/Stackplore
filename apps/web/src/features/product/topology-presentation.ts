import type { ProductRelationship, RelationshipEvidence } from '@stackplore/api-client';
import type { TextMessageKey } from '../../i18n';
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
