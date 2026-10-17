import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useLiveObservation } from './use-live-observation';
class Stream extends EventTarget {
  static latest: Stream;
  close = vi.fn();
  constructor() {
    super();
    Stream.latest = this;
  }
}
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});
describe('bounded live observation refresh', () => {
  it('coalesces stream bursts and replay resets, then closes on unmount', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', Stream);
    const { result, unmount } = renderHook(() => useLiveObservation('source_demo'));
    act(() => {
      for (let index = 0; index < 100; index++)
        Stream.latest.dispatchEvent(new Event('health.snapshot.v1'));
    });
    expect(result.current.revision).toBe(0);
    act(() => {
      vi.advanceTimersByTime(4999);
    });
    expect(result.current.revision).toBe(0);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current.revision).toBe(100);
    act(() => {
      Stream.latest.dispatchEvent(new Event('stream.reset.v1'));
    });
    expect(result.current.status).toBe('stale');
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(result.current.revision).toBe(101);
    unmount();
    expect(Stream.latest.close).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  });
});
