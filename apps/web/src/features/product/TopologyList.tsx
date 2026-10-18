import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import { useState } from 'react';
import { useLocale } from '../../i18n';
import { topologyListPageSize, topologyResourceHref } from './topology-model';
import { evidence, relationshipKeys, topologyResourceState } from './topology-presentation';

export function TopologyList({
  resources,
  relationships,
}: {
  readonly resources: readonly ProductResource[];
  readonly relationships: readonly ProductRelationship[];
}) {
  const { t, formatDate } = useLocale();
  const [nodePage, setNodePage] = useState(0);
  const [edgePage, setEdgePage] = useState(0);
  const nodeSlice = pageSlice(resources, nodePage);
  const edgeSlice = pageSlice(relationships, edgePage);
  const names = new Map(resources.map((resource) => [resource.id, resource]));
  const connectedIds = new Set(relationships.flatMap((edge) => [edge.from, edge.to]));
  const direction = (edge: ProductRelationship) => t(`topology.direction.${edge.direction}`);
  return (
    <details className="topology-list-view" open>
      <summary>{t('topology.nodeList')}</summary>
      <section className="topology-accessible-list" aria-label={t('topology.nodes')}>
        <h2>{t('topology.nodes')}</h2>
        <ul>
          {nodeSlice.items.map((resource) => (
            <li key={resource.id}>
              <a href={topologyResourceHref(resource)}>{resource.name}</a>
              <bdi dir="ltr">
                {resource.provider} · {resource.kind} · {resource.scope_name ?? resource.scope_kind}
              </bdi>
              <span>{t(`product.health.${resource.health ?? 'unknown'}`)}</span>
              {topologyResourceState(resource, t) === undefined ? null : (
                <span>{topologyResourceState(resource, t)}</span>
              )}
              {connectedIds.has(resource.id) ? null : <span>{t('topology.isolated')}</span>}
            </li>
          ))}
        </ul>
        <Pager
          current={nodeSlice.page}
          count={nodeSlice.count}
          onChange={setNodePage}
          label={t('topology.nodes')}
        />
      </section>
      <section className="topology-accessible-list" aria-label={t('topology.edges')}>
        <h2>{t('topology.edges')}</h2>
        <ul>
          {edgeSlice.items.map((edge) => (
            <li key={edge.id} className="topology-relationship">
              <div className="topology-endpoints">
                {endpoint(edge.from, names)}
                <span>
                  {t(relationshipKeys[edge.kind])} · {direction(edge)}
                </span>
                {endpoint(edge.to, names)}
              </div>
              <span>
                {t('product.topology.confidence', {
                  count: Math.round(edge.confidence * 100),
                })}
              </span>
              {edge.kind === 'member_of' ? <p>{t('topology.noDependency')}</p> : null}
              <details>
                <summary>{t('topology.evidence')}</summary>
                {evidence(edge).length === 0 ? (
                  <p>{t('topology.noEvidence')}</p>
                ) : (
                  <ul>
                    {evidence(edge).map((item) => (
                      <li key={item.id}>
                        <span>
                          {t(
                            item.certainty === 'explicit'
                              ? 'topology.explicit'
                              : 'topology.inferred',
                          )}{' '}
                          · {t(`topology.provenance.${item.provenance}`)}
                        </span>
                        <span>
                          {t('topology.provider')}:{' '}
                          <bdi dir="ltr">
                            {item.provider} · {item.source_id}
                          </bdi>
                        </span>
                        <span>
                          {t('product.topology.confidence', {
                            count: Math.round(item.confidence * 100),
                          })}
                        </span>
                        <span>
                          {t('topology.observed')}:{' '}
                          <time dateTime={item.observed_at}>
                            {formatDate(Date.parse(item.observed_at), {
                              dateStyle: 'medium',
                              timeStyle: 'medium',
                            })}
                          </time>
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </details>
            </li>
          ))}
        </ul>
        <Pager
          current={edgeSlice.page}
          count={edgeSlice.count}
          onChange={setEdgePage}
          label={t('topology.edges')}
        />
      </section>
    </details>
  );
}
function endpoint(id: string, resources: ReadonlyMap<string, ProductResource>) {
  const resource = resources.get(id);
  return resource === undefined ? (
    <bdi dir="ltr">{id}</bdi>
  ) : (
    <a href={topologyResourceHref(resource)}>{resource.name}</a>
  );
}
function Pager({
  current,
  count,
  label,
  onChange,
}: {
  readonly current: number;
  readonly count: number;
  readonly label: string;
  readonly onChange: (value: number) => void;
}) {
  const { t } = useLocale();
  if (count <= 1) return null;
  return (
    <nav className="topology-pager" aria-label={label}>
      <button
        disabled={current === 0}
        onClick={() => {
          onChange(current - 1);
        }}
        type="button"
      >
        {t('topology.previous')}
      </button>
      <span role="status">{t('topology.page', { page: current + 1, count })}</span>
      <button
        disabled={current + 1 >= count}
        onClick={() => {
          onChange(current + 1);
        }}
        type="button"
      >
        {t('topology.next')}
      </button>
    </nav>
  );
}
function pageSlice<T>(items: readonly T[], requested: number) {
  const count = Math.max(1, Math.ceil(items.length / topologyListPageSize));
  const page = Math.min(requested, count - 1);
  return {
    page,
    count,
    items: items.slice(page * topologyListPageSize, (page + 1) * topologyListPageSize),
  };
}
