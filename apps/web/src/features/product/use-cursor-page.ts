import { useCallback, useEffect, useRef, useState } from 'react';

export interface CursorPage<Item> {
  readonly items: readonly Item[];
  readonly page: {
    readonly has_more: boolean;
    readonly next_cursor?: string;
    readonly limit: number;
  };
  readonly availability?: string;
  readonly completeness?: 'complete' | 'partial';
  readonly reason_code?: string;
}

export type PageLoad<Page> =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error' }
  | { readonly kind: 'ready'; readonly data: Page };

export const explorationItemLimit = 1000;

/** Each view owns a bounded set. Changing its query cancels every pending page. */
export function useCursorPage<Item extends { readonly id: string }, Page extends CursorPage<Item>>(
  load: (signal: AbortSignal, cursor?: string) => Promise<Page>,
) {
  const [state, setState] = useState<PageLoad<Page>>({ kind: 'loading' });
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);
  const [truncated, setTruncated] = useState(false);
  const generation = useRef(0);
  const active = useRef<AbortController | null>(null);
  const pending = useRef(false);
  const current = useRef(state);
  current.current = state;

  useEffect(() => {
    const controller = new AbortController();
    active.current?.abort();
    active.current = controller;
    const version = ++generation.current;
    setState({ kind: 'loading' });
    setLoadingMore(false);
    setMoreFailed(false);
    setTruncated(false);
    pending.current = false;
    void load(controller.signal).then(
      (data) => {
        if (!controller.signal.aborted && version === generation.current)
          setState({ kind: 'ready', data });
      },
      () => {
        if (!controller.signal.aborted && version === generation.current)
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
      active.current?.abort();
      generation.current++;
    };
  }, [load]);

  const loadMore = useCallback(async () => {
    const value = current.current;
    if (
      value.kind !== 'ready' ||
      !value.data.page.has_more ||
      pending.current ||
      value.data.items.length >= explorationItemLimit
    )
      return;
    const cursor = value.data.page.next_cursor;
    if (cursor === undefined) return;
    const version = generation.current;
    pending.current = true;
    const controller = new AbortController();
    active.current?.abort();
    active.current = controller;
    setLoadingMore(true);
    setMoreFailed(false);
    try {
      const next = await load(controller.signal, cursor);
      if (controller.signal.aborted || version !== generation.current) return;
      if (next.page.has_more && next.page.next_cursor === cursor)
        throw new Error('Page did not advance.');
      if (
        new Set([...value.data.items, ...next.items].map((item) => item.id)).size >
        explorationItemLimit
      )
        setTruncated(true);
      setState((previous) => {
        if (previous.kind !== 'ready' || previous.data.page.next_cursor !== cursor) return previous;
        const items = new Map(previous.data.items.map((item) => [item.id, item]));
        for (const item of next.items) items.set(item.id, item);
        const keepStale =
          previous.data.availability === 'stale' && next.availability === 'available';
        const keepPartial =
          previous.data.completeness === 'partial' && next.completeness === 'complete';
        return {
          kind: 'ready',
          data: {
            ...next,
            items: [...items.values()].slice(0, explorationItemLimit),
            ...(keepStale ? { availability: 'stale' } : {}),
            ...(keepPartial ? { completeness: 'partial' } : {}),
            ...((keepStale || keepPartial) && previous.data.reason_code !== undefined
              ? { reason_code: previous.data.reason_code }
              : {}),
          },
        };
      });
    } catch {
      if (!controller.signal.aborted && version === generation.current) setMoreFailed(true);
    } finally {
      if (version === generation.current) {
        pending.current = false;
        setLoadingMore(false);
      }
    }
  }, [load]);
  return {
    state,
    setState,
    loadMore,
    loadingMore,
    moreFailed,
    capped:
      truncated ||
      (state.kind === 'ready' &&
        state.data.items.length >= explorationItemLimit &&
        state.data.page.has_more),
  };
}
