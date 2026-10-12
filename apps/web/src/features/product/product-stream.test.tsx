import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useProductStream } from './product-stream';

class FakeEventSource extends EventTarget {
  static instances: FakeEventSource[] = [];

  readonly close = vi.fn();
  readonly url: string;
  readonly withCredentials: boolean;

  constructor(url: string | URL, options?: EventSourceInit) {
    super();
    this.url = String(url);
    this.withCredentials = options?.withCredentials ?? false;
    FakeEventSource.instances.push(this);
  }
}

afterEach(() => {
  FakeEventSource.instances = [];
  vi.unstubAllGlobals();
});

describe('product stream', () => {
  it('tracks connection, snapshots, reset, and cleanup', () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result, unmount } = renderHook(() => useProductStream('events', 'source_demo'));
    const stream = FakeEventSource.instances[0];
    expect(stream?.url).toBe('/api/v1/streams/events?source_id=source_demo');
    expect(stream?.withCredentials).toBe(true);

    act(() => {
      stream?.dispatchEvent(new Event('open'));
    });
    expect(result.current).toEqual({ revision: 0, status: 'connected' });
    act(() => {
      stream?.dispatchEvent(new Event('events.snapshot.v1'));
    });
    expect(result.current).toEqual({ revision: 1, status: 'connected' });
    act(() => {
      stream?.dispatchEvent(new Event('stream.reset.v1'));
    });
    expect(result.current.status).toBe('stale');
    act(() => {
      stream?.dispatchEvent(new Event('error'));
    });
    expect(result.current.status).toBe('disconnected');

    unmount();
    expect(stream?.close).toHaveBeenCalledOnce();
  });

  it('reports unavailable when the browser lacks stream support', () => {
    vi.stubGlobal('EventSource', undefined);
    const { result } = renderHook(() => useProductStream('health'));
    expect(result.current.status).toBe('unavailable');
  });
});
