import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useCursorPage } from './use-cursor-page';
type Page = {
  items: { id: string; label: string }[];
  page: { limit: number; has_more: boolean; next_cursor?: string };
};
describe('live cursor refresh', () => {
  it('reloads the user-loaded page budget with fresh cursors and no lost rows', async () => {
    let version = 1;
    const load = vi.fn((_signal: AbortSignal, cursor?: string): Promise<Page> =>
      Promise.resolve(
        cursor === undefined
          ? {
              items: [{ id: 'a', label: `a${String(version)}` }],
              page: { limit: 100, has_more: true, next_cursor: 'next' },
            }
          : {
              items: [{ id: 'b', label: `b${String(version)}` }],
              page: { limit: 100, has_more: false },
            },
      ),
    );
    const { result, rerender } = renderHook(({ revision }) => useCursorPage(load, revision), {
      initialProps: { revision: 0 },
    });
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    await act(() => result.current.loadMore());
    expect(load).toHaveBeenCalledTimes(2);
    version = 2;
    rerender({ revision: 1 });
    await waitFor(() => {
      expect(load).toHaveBeenCalledTimes(4);
    });
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    if (result.current.state.kind !== 'ready') throw new Error('unreachable');
    expect(result.current.state.data.items.map((item) => item.label)).toEqual(['a2', 'b2']);
    expect(result.current.state.data.page.has_more).toBe(false);
  });
  it('cancels replaced filters and never follows an old query cursor', async () => {
    let resolve: ((page: Page) => void) | undefined;
    let priorSignal: AbortSignal | undefined;
    const oldLoad = vi.fn((signal: AbortSignal) => {
      priorSignal = signal;
      return new Promise<Page>((done) => {
        resolve = done;
      });
    });
    const newLoad = vi.fn((): Promise<Page> =>
      Promise.resolve({
        items: [{ id: 'new', label: 'New filter' }],
        page: { limit: 100, has_more: false },
      }),
    );
    const { result, rerender } = renderHook(({ load }) => useCursorPage(load, 0), {
      initialProps: { load: oldLoad },
    });
    rerender({ load: newLoad });
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    await act(async () => {
      resolve?.({
        items: [{ id: 'old', label: 'Old' }],
        page: { limit: 100, has_more: true, next_cursor: 'old_cursor' },
      });
      await Promise.resolve();
    });
    expect(priorSignal?.aborted).toBe(true);
    expect(oldLoad).toHaveBeenCalledOnce();
    if (result.current.state.kind !== 'ready') throw new Error('unreachable');
    expect(result.current.state.data.items[0]?.id).toBe('new');
  });
});
