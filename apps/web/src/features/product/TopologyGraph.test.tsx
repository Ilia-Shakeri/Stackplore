import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import axe from 'axe-core';
import { beforeEach, describe, expect, it } from 'vitest';

import { TopologyGraph } from './TopologyGraph';
import { LocaleProvider } from '../../i18n';

const firstId = `r1_${'A'.repeat(43)}`;
const secondId = `r1_${'B'.repeat(43)}`;
const resources: ProductResource[] = [
  {
    id: firstId,
    provider: 'demo',
    source_id: 'source_demo',
    environment_id: 'env_demo',
    kind: 'demo.service',
    scope_kind: 'project',
    scope_name: 'shop',
    name: 'Gateway',
    synthetic: true,
    observed_at: '2026-08-16T08:00:00Z',
    health: 'healthy',
  },
  {
    id: secondId,
    provider: 'demo',
    source_id: 'source_demo',
    environment_id: 'env_demo',
    kind: 'demo.database',
    scope_kind: 'project',
    scope_name: 'shop',
    name: 'Database',
    synthetic: true,
    observed_at: '2026-08-16T08:00:00Z',
    health: 'degraded',
  },
];
const relationships: ProductRelationship[] = [
  {
    id: `rel1_${'C'.repeat(43)}`,
    from: firstId,
    to: secondId,
    kind: 'depends_on',
    direction: 'source_to_target',
    observed_at: '2026-08-16T08:00:00Z',
    synthetic: true,
    confidence: 1,
    evidence: [
      {
        id: `ev1_${'D'.repeat(43)}`,
        provider: 'demo',
        source_id: 'source_demo',
        provenance: 'declared_dependency',
        certainty: 'explicit',
        confidence: 1,
        observed_at: '2026-08-16T08:00:00Z',
        synthetic: true,
      },
    ],
  },
];

describe('topology graph', () => {
  beforeEach(() => {
    window.history.replaceState(null, '', '/topology');
  });
  it('offers keyboard controls, node links, and search filtering', async () => {
    const user = userEvent.setup();
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.topology-test.locale">
        <TopologyGraph relationships={relationships} resources={resources} />
      </LocaleProvider>,
    );

    expect(
      screen.getAllByRole('group', { name: 'Interactive infrastructure topology' }),
    ).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Zoom in' })).not.toBeNull();
    expect(screen.getAllByRole('link', { name: /Gateway/u }).length).toBeGreaterThan(0);

    await user.type(screen.getByRole('searchbox', { name: 'Find node' }), 'missing');
    expect(screen.getByRole('status').textContent).toBe('No nodes match these filters.');
  });

  it('keeps isolated nodes navigable and exposes exact relationship evidence', async () => {
    const isolated: ProductResource = {
      ...required(resources[0]),
      id: `r1_${'D'.repeat(43)}`,
      name: 'Isolated queue',
      health: 'unknown',
    };
    const user = userEvent.setup();
    const { container } = render(
      <LocaleProvider defaultLocale="en" storageKey="topology-isolated">
        <TopologyGraph relationships={relationships} resources={[...resources, isolated]} />
      </LocaleProvider>,
    );
    const nodeList = screen.getByRole('region', { name: 'Nodes' });
    const link = within(nodeList).getByRole('link', { name: 'Isolated queue' });
    expect(link.getAttribute('href')).toContain(encodeURIComponent(isolated.id));
    expect(within(nodeList).getByText('No relationship in the loaded view')).not.toBeNull();
    await user.click(screen.getByText('Evidence'));
    expect(screen.getByText('Explicit · Declared dependency')).not.toBeNull();
    expect(screen.getByText('depends on · Source to target')).not.toBeNull();
    const path = container.querySelector('.topology-edge');
    expect(path?.getAttribute('marker-end')).toContain('url(');
    expect(path?.hasAttribute('marker-start')).toBe(false);
    await user.selectOptions(screen.getByRole('combobox', { name: 'Select a node' }), isolated.id);
    expect(
      within(screen.getByRole('complementary', { name: 'Selected resource' }))
        .getByRole('link', { name: 'Open resource' })
        .getAttribute('href'),
    ).toContain(isolated.id);
  });

  it('shows distinct inferred undirected edges and two arrowheads for bidirectional edges', () => {
    const base = required(relationships[0]);
    const mixed: ProductRelationship[] = [
      {
        ...base,
        direction: 'undirected',
        kind: 'member_of',
        evidence: base.evidence.map((item) => ({
          ...item,
          certainty: 'inferred',
          provenance: 'network_membership',
        })),
      },
      { ...base, id: `rel1_${'E'.repeat(43)}`, direction: 'bidirectional' },
    ];
    const { container } = render(
      <LocaleProvider defaultLocale="en" storageKey="topology-directions">
        <TopologyGraph relationships={mixed} resources={resources} />
      </LocaleProvider>,
    );
    const paths = container.querySelectorAll('.topology-edge');
    expect(paths[0]?.getAttribute('class')).toContain('topology-edge--inferred');
    expect(paths[0]?.hasAttribute('marker-end')).toBe(false);
    expect(paths[1]?.getAttribute('marker-start')).toContain('url(');
    expect(paths[1]?.getAttribute('marker-end')).toContain('url(');
    expect(
      screen.getByText('Shared membership does not prove an application dependency.'),
    ).not.toBeNull();
  });

  it('restores shareable filters and responds to browser history', async () => {
    window.history.replaceState(null, '', '/topology?topology_health=healthy&topology_group=kind');
    const user = userEvent.setup();
    render(
      <LocaleProvider defaultLocale="en" storageKey="topology-url">
        <TopologyGraph relationships={relationships} resources={resources} />
      </LocaleProvider>,
    );
    expect(screen.getByRole<HTMLSelectElement>('combobox', { name: 'Health' }).value).toBe(
      'healthy',
    );
    expect(screen.queryByRole('link', { name: 'Database' })).toBeNull();
    await user.selectOptions(screen.getByRole('combobox', { name: 'Health' }), 'degraded');
    expect(new URLSearchParams(window.location.search).get('topology_health')).toBe('degraded');
    act(() => {
      window.history.replaceState(null, '', '/topology?topology_health=healthy');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(screen.getByRole<HTMLSelectElement>('combobox', { name: 'Health' }).value).toBe(
      'healthy',
    );
  });

  it('bounds graph and list rendering while retaining all loaded nodes through pages', async () => {
    const many = Array.from({ length: 201 }, (_, index): ProductResource => ({
      ...required(resources[0]),
      id: `r1_${String(index).padStart(43, 'A')}`,
      name: `Node ${String(index)}`,
    }));
    const user = userEvent.setup();
    const { container } = render(
      <LocaleProvider defaultLocale="en" storageKey="topology-budget">
        <TopologyGraph relationships={[]} resources={many} />
      </LocaleProvider>,
    );
    expect(container.querySelectorAll('.topology-node')).toHaveLength(200);
    expect(
      within(screen.getByRole('region', { name: 'Nodes' })).getAllByRole('listitem'),
    ).toHaveLength(50);
    expect(
      screen.getByText(
        'Graph shows 200 of 201 matching nodes. All loaded nodes remain in the list.',
      ),
    ).not.toBeNull();
    const nodeNavigation = screen.getByRole('navigation', { name: 'Nodes' });
    for (let index = 0; index < 4; index += 1)
      await user.click(within(nodeNavigation).getByRole('button', { name: 'Next' }));
    expect(
      within(screen.getByRole('region', { name: 'Nodes' })).getAllByRole('listitem'),
    ).toHaveLength(1);
    expect(screen.getByRole('link', { name: 'Node 200' })).not.toBeNull();
  });

  it('restores a full-length scope filter without cutting its identity', () => {
    const scopeName = 'n'.repeat(253);
    const scope = `namespace: ${scopeName}`;
    window.history.replaceState(null, '', `/topology?topology_scope=${encodeURIComponent(scope)}`);
    render(
      <LocaleProvider defaultLocale="en" storageKey="topology-long-scope">
        <TopologyGraph
          relationships={[]}
          resources={[
            { ...required(resources[0]), scope_kind: 'namespace', scope_name: scopeName },
          ]}
        />
      </LocaleProvider>,
    );
    expect(screen.getByRole<HTMLSelectElement>('combobox', { name: 'Scope' }).value).toBe(scope);
    expect(screen.getAllByRole('link', { name: 'Gateway' }).length).toBeGreaterThan(0);
  });

  it.each(['en', 'fa'] as const)(
    'keeps graph controls and equivalent list accessible in %s',
    async (locale) => {
      const { container } = render(
        <LocaleProvider defaultLocale={locale} storageKey={`topology-axe-${locale}`}>
          <main>
            <h1>Topology</h1>
            <TopologyGraph relationships={relationships} resources={resources} />
          </main>
        </LocaleProvider>,
      );
      const result = await axe.run(container, { rules: { 'color-contrast': { enabled: false } } });
      expect(result.violations).toEqual([]);
    },
  );
});

function required<T>(value: T | undefined): T {
  if (value === undefined) throw new Error('Fixture missing.');
  return value;
}
