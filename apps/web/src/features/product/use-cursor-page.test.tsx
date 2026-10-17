import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useCursorPage, type CursorPage } from './use-cursor-page';

type Page = CursorPage<{ readonly id: string }>;
const page = (ids: readonly string[], cursor?: string): Page => ({
  items: ids.map((id) => ({ id })),
  page: {
    limit: 100,
    has_more: cursor !== undefined,
    ...(cursor === undefined ? {} : { next_cursor: cursor }),
  },
});

describe('bounded exploration pagination', () => {
  it('keeps loaded rows, deduplicates and keeps partial evidence', async () => {
    const load = vi
      .fn()
      .mockResolvedValueOnce({
        ...page(['one', 'two'], 'next'),
        availability: 'stale',
        completeness: 'partial',
        reason_code: 'capture.partial',
      })
      .mockResolvedValueOnce({
        ...page(['two', 'three']),
        availability: 'available',
        completeness: 'complete',
      });
    const { result } = renderHook(() => useCursorPage(load));
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.state).toMatchObject({
      kind: 'ready',
      data: {
        items: [{ id: 'one' }, { id: 'two' }, { id: 'three' }],
        availability: 'stale',
        completeness: 'partial',
      },
    });
  });

  it('aborts an old cursor on a query change and ignores its late reply', async () => {
    let finish!: (value: Page) => void;
    const first = vi
      .fn()
      .mockResolvedValueOnce(page(['old'], 'next'))
      .mockImplementationOnce(
        () =>
          new Promise<Page>((resolve) => {
            finish = resolve;
          }),
      );
    const second = vi.fn().mockResolvedValue(page(['current']));
    const { result, rerender } = renderHook(({ load }) => useCursorPage(load), {
      initialProps: { load: first },
    });
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    act(() => {
      void result.current.loadMore();
    });
    const signal = first.mock.calls[1]?.[0] as AbortSignal;
    rerender({ load: second });
    expect(signal.aborted).toBe(true);
    await waitFor(() => {
      expect(result.current.state).toMatchObject({
        kind: 'ready',
        data: { items: [{ id: 'current' }] },
      });
    });
    await act(async () => {
      finish(page(['late']));
      await Promise.resolve();
    });
    expect(result.current.state).toMatchObject({
      kind: 'ready',
      data: { items: [{ id: 'current' }] },
    });
  });

  it('rejects nonadvancing cursors and keeps a retryable page', async () => {
    const load = vi.fn().mockResolvedValue(page(['one'], 'same'));
    const { result } = renderHook(() => useCursorPage(load));
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.moreFailed).toBe(true);
    expect(result.current.state).toMatchObject({ kind: 'ready', data: { items: [{ id: 'one' }] } });
  });

  it('stops at 1000 rows even when more history exists', async () => {
    const load = vi
      .fn()
      .mockResolvedValueOnce(
        page(
          Array.from({ length: 900 }, (_, index) => String(index)),
          'next',
        ),
      )
      .mockResolvedValueOnce(
        page(
          Array.from({ length: 200 }, (_, index) => String(index + 900)),
          'later',
        ),
      );
    const { result } = renderHook(() => useCursorPage(load));
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.capped).toBe(true);
    await act(async () => {
      await result.current.loadMore();
    });
    expect(load).toHaveBeenCalledTimes(2);
    if (result.current.state.kind === 'ready')
      expect(result.current.state.data.items).toHaveLength(1000);
  });
  it('marks a truncated final page even when the server has no further page', async () => {
    const load = vi
      .fn()
      .mockResolvedValueOnce(
        page(
          Array.from({ length: 950 }, (_, index) => String(index)),
          'last',
        ),
      )
      .mockResolvedValueOnce(page(Array.from({ length: 100 }, (_, index) => String(index + 940))));
    const { result } = renderHook(() => useCursorPage(load));
    await waitFor(() => {
      expect(result.current.state.kind).toBe('ready');
    });
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.capped).toBe(true);
    expect(result.current.state).toMatchObject({
      kind: 'ready',
      data: { page: { has_more: false } },
    });
    if (result.current.state.kind === 'ready')
      expect(result.current.state.data.items).toHaveLength(1000);
  });
});
