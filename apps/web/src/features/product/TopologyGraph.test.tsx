import type { ProductRelationship, ProductResource } from '@stackplore/api-client';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

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
  },
];

describe('topology graph', () => {
  it('offers keyboard controls, node links, and search filtering', async () => {
    const user = userEvent.setup();
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.topology-test.locale">
        <TopologyGraph relationships={relationships} resources={resources} />
      </LocaleProvider>,
    );

    expect(
      screen.getAllByRole('img', { name: 'Interactive infrastructure topology' }),
    ).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Zoom in' })).not.toBeNull();
    expect(screen.getByRole('link', { name: /Gateway/u })).not.toBeNull();

    await user.type(screen.getByRole('searchbox', { name: 'Find node' }), 'missing');
    expect(screen.getByRole('status').textContent).toBe('No nodes match these filters.');
  });
});
