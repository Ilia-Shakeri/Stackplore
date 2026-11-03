import type { ProductResource, ProductResourcePage, SourceSummary } from '@stackplore/api-client';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AssistantWorkspacePage } from './AssistantWorkspacePage';
import { AssistantProvider } from './assistant-context';
import { controlledConversationApi, scope } from './conversation.test-fixtures';
import { AuthProvider } from '../auth/auth-context';
import { ProductProvider } from '../product/product-context';
import { createProductApi, type ProductApi } from '../product/product-api';
import { createAuthenticatedAuthStore, createResolvedAuthStore } from '../../test/auth';
import { LocaleProvider, translateText } from '../../i18n';

const observed = {
  availability: 'available',
  completeness: 'complete',
  observed_at: '2026-10-28T10:15:00Z',
} as const;
const resource: ProductResource = {
  id: scope.resourceId,
  source_id: scope.sourceId,
  provider: 'docker',
  kind: 'docker.container',
  environment_id: 'env_one',
  scope_kind: 'project',
  name: 'Selected service',
  synthetic: true,
  observed_at: observed.observed_at,
};
const source: SourceSummary = {
  ...observed,
  id: scope.sourceId,
  provider: 'docker',
  name: 'Source one',
  synthetic: true,
  resource_count: 1,
  health: { healthy: 1, degraded: 0, unhealthy: 0, unknown: 0 },
};
const page = (items: ProductResource[]): ProductResourcePage => ({
  ...observed,
  items,
  page: { has_more: false, limit: 10 },
});
function fixture() {
  return {
    ...createProductApi({ request: () => Promise.reject(new Error('Unexpected workspace read.')) }),
    listSources: vi.fn<ProductApi['listSources']>().mockResolvedValue([source]),
    listResources: vi.fn<ProductApi['listResources']>().mockResolvedValue(page([resource])),
    getResource: vi.fn<ProductApi['getResource']>().mockResolvedValue(resource),
  };
}
function show(product: ProductApi, guest = false) {
  const controlled = controlledConversationApi();
  const view = render(
    <LocaleProvider>
      <AuthProvider
        store={
          guest
            ? createResolvedAuthStore({
                status: 'guest_demo',
                fixtureVersion: 'demo-v2',
                scenarioStep: 0,
                observedAt: observed.observed_at,
              })
            : createAuthenticatedAuthStore()
        }
      >
        <ProductProvider api={product}>
          <AssistantProvider api={controlled.api}>
            <AssistantWorkspacePage />
          </AssistantProvider>
        </ProductProvider>
      </AuthProvider>
    </LocaleProvider>,
  );
  return { ...view, assistant: controlled.api };
}
afterEach(() => {
  window.history.replaceState(null, '', '/');
});

describe('dedicated assistant workspace', () => {
  it('loads bounded choices, requires a selection, and clears the question when changing resource', async () => {
    const user = userEvent.setup();
    const product = fixture();
    const { assistant } = show(product);
    await user.click(await screen.findByRole('button', { name: resource.name }));
    const input = await screen.findByRole('textbox', { name: 'Question about this resource' });
    expect(product.listResources).toHaveBeenCalledWith(
      expect.objectContaining({ sourceId: source.id, limit: 10 }),
    );
    expect(product.getResource).toHaveBeenCalledWith(
      resource.id,
      expect.any(AbortSignal),
      source.id,
    );
    await user.type(input, 'Temporary question');
    expect(assistant.admit).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Choose another resource' }));
    expect(screen.queryByRole('textbox', { name: 'Question about this resource' })).toBeNull();
    await user.click(await screen.findByRole('button', { name: resource.name }));
    expect(
      (
        await screen.findByRole<HTMLTextAreaElement>('textbox', {
          name: 'Question about this resource',
        })
      ).value,
    ).toBe('');
    expect(assistant.admit).not.toHaveBeenCalled();
  });

  it('rejects a resource returned for a different source', async () => {
    const product = fixture();
    product.getResource.mockResolvedValue({ ...resource, source_id: 'another_source' });
    const { assistant } = show(product);
    fireEvent.click(await screen.findByRole('button', { name: resource.name }));
    await screen.findByText(translateText('en', 'product.error'));
    expect(screen.queryByRole('textbox', { name: 'Question about this resource' })).toBeNull();
    expect(assistant.admit).not.toHaveBeenCalled();
  });

  it('ignores a selected-resource completion after the selection is cleared', async () => {
    let finish: ((value: ProductResource) => void) | undefined;
    const product = fixture();
    product.getResource.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    show(product);
    fireEvent.click(await screen.findByRole('button', { name: resource.name }));
    await waitFor(() => {
      expect(product.getResource).toHaveBeenCalledOnce();
    });
    const signal = product.getResource.mock.calls[0]?.[1];
    fireEvent.click(screen.getByRole('button', { name: 'Choose another resource' }));
    await act(async () => {
      finish?.(resource);
      await Promise.resolve();
    });
    expect(signal?.aborted).toBe(true);
    expect(screen.queryByRole('textbox', { name: 'Question about this resource' })).toBeNull();
  });

  it('replaces resource pages instead of appending an endless list', async () => {
    const other = { ...resource, id: `r1_${'B'.repeat(43)}`, name: 'Next service' };
    const product = fixture();
    product.listResources
      .mockResolvedValueOnce({
        ...page([resource]),
        page: { has_more: true, next_cursor: 'next_page', limit: 10 },
      })
      .mockResolvedValue(page([other]));
    show(product);
    await screen.findByRole('button', { name: resource.name });
    fireEvent.click(screen.getByRole('button', { name: 'Next page' }));
    await screen.findByRole('button', { name: other.name });
    expect(screen.queryByRole('button', { name: resource.name })).toBeNull();
    expect(product.listResources).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: 'next_page', limit: 10 }),
    );
  });

  it('retries the same search from its first page after a failed read', async () => {
    const product = fixture();
    product.listResources.mockRejectedValueOnce(new Error('Temporary read failure.'));
    show(product);
    await screen.findByText(translateText('en', 'product.error'));
    fireEvent.click(screen.getByRole('button', { name: 'Find resources' }));
    await screen.findByRole('button', { name: resource.name });
    expect(product.listResources).toHaveBeenCalledTimes(2);
  });

  it('shows the guest limit without opening private or inventory reads', async () => {
    const product = fixture();
    const { assistant } = show(product, true);
    expect(await screen.findByText(/unavailable in the guest demonstration/u)).not.toBeNull();
    expect(product.listSources).not.toHaveBeenCalled();
    expect(product.listResources).not.toHaveBeenCalled();
    expect(assistant.admit).not.toHaveBeenCalled();
  });
});
