import type {
  ProductMetricHistoryResult,
  ProductMetricResult,
  SourceSummary,
} from '@stackplore/api-client';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { ProductOverviewPage } from './ProductPages';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';

vi.mock('./MetricTrend', () => ({ MetricTrend: () => null }));
const at = '2026-08-16T08:00:00Z';
const observation = {
  availability: 'available',
  completeness: 'complete',
  observed_at: at,
} as const;
const metrics: ProductMetricResult = { ...observation, items: [] };
const history: ProductMetricHistoryResult = { ...observation, items: [] };
const sources: readonly SourceSummary[] = [
  {
    ...observation,
    id: 'source_demo',
    provider: 'demo',
    name: 'Demo',
    synthetic: true,
    resource_count: 0,
    health: { healthy: 0, degraded: 0, unhealthy: 0, unknown: 0 },
  },
  {
    ...observation,
    id: 'source_docker',
    provider: 'docker',
    name: 'Docker',
    synthetic: false,
    resource_count: 0,
    health: { healthy: 0, degraded: 0, unhealthy: 0, unknown: 0 },
  },
];
class Stream extends EventTarget {
  static latest: Stream;
  close = vi.fn();
  constructor() {
    super();
    Stream.latest = this;
  }
}
function fixture() {
  const api = {
    ...createProductApi({
      request: () => Promise.reject(new Error('Unexpected overview request.')),
    }),
    listSources: vi.fn<ProductApi['listSources']>().mockResolvedValue(sources),
    listMetrics: vi.fn<ProductApi['listMetrics']>().mockResolvedValue(metrics),
    listMetricHistory: vi.fn<ProductApi['listMetricHistory']>().mockResolvedValue(history),
    listAlerts: vi
      .fn<ProductApi['listAlerts']>()
      .mockResolvedValue({ items: [], page: { limit: 100, has_more: false } }),
    listEnvironments: vi
      .fn<ProductApi['listEnvironments']>()
      .mockResolvedValue({ ...observation, items: [], page: { limit: 100, has_more: false } }),
    listEvents: vi
      .fn<ProductApi['listEvents']>()
      .mockResolvedValue({ ...observation, items: [], page: { limit: 10, has_more: false } }),
    listResources: vi
      .fn<ProductApi['listResources']>()
      .mockResolvedValue({ ...observation, items: [], page: { limit: 100, has_more: false } }),
  };
  return api;
}
function deferred<Value>() {
  let resolve!: (value: Value) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<Value>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
function mount(api: ProductApi, sourceId = 'source_demo') {
  return (
    <LocaleProvider defaultLocale="en" storageKey="overview-refresh-test">
      <ProductProvider api={api} sourceId={sourceId} onSourceChange={() => {}}>
        <ProductOverviewPage />
      </ProductProvider>
    </LocaleProvider>
  );
}
async function settle() {
  await act(async () => {
    await Promise.resolve();
  });
}
function setup() {
  vi.useFakeTimers();
  vi.stubGlobal('EventSource', Stream);
}
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  window.history.replaceState(null, '', '/');
});

describe('overview bounded refresh', () => {
  it('loads one exact fixture-clock batch, ignores the initial snapshot and coalesces later bursts', async () => {
    setup();
    const api = fixture();
    const { unmount } = render(mount(api));
    await settle();
    expect(screen.getByRole('region', { name: 'Current source metrics' })).not.toBeNull();
    expect(api.listSources).toHaveBeenCalledTimes(1);
    expect(api.listMetrics).toHaveBeenCalledTimes(1);
    expect(api.listMetricHistory).toHaveBeenCalledTimes(1);
    expect(api.listMetricHistory.mock.calls[0]?.[0]).toMatchObject({
      from: '2026-08-16T07:00:00.001Z',
      to: '2026-08-16T08:00:00.001Z',
    });
    act(() => {
      Stream.latest.dispatchEvent(new Event('metrics.snapshot.v1'));
      vi.advanceTimersByTime(5000);
    });
    await settle();
    expect(api.listMetrics).toHaveBeenCalledTimes(1);
    act(() => {
      for (let index = 0; index < 100; index++)
        Stream.latest.dispatchEvent(new Event('metrics.snapshot.v1'));
    });
    act(() => {
      vi.advanceTimersByTime(4999);
    });
    expect(api.listMetrics).toHaveBeenCalledTimes(1);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    await settle();
    expect(api.listMetrics).toHaveBeenCalledTimes(2);
    expect(api.listSources).toHaveBeenCalledTimes(2);
    expect(api.listMetricHistory).toHaveBeenCalledTimes(2);
    act(() => {
      Stream.latest.dispatchEvent(new Event('stream.reset.v1'));
    });
    unmount();
    expect(Stream.latest.close).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('lets an active read settle before one queued stream refresh', async () => {
    setup();
    const api = fixture();
    const slow = deferred<ProductMetricResult>();
    api.listMetrics.mockImplementationOnce(() => slow.promise);
    render(mount(api));
    const signal = api.listMetrics.mock.calls[0]?.[0]?.signal;
    act(() => {
      Stream.latest.dispatchEvent(new Event('metrics.snapshot.v1'));
    });
    act(() => {
      for (let index = 0; index < 3; index++)
        Stream.latest.dispatchEvent(new Event('metrics.snapshot.v1'));
      vi.advanceTimersByTime(5000);
    });
    expect(api.listMetrics).toHaveBeenCalledTimes(1);
    expect(signal?.aborted).toBe(false);
    await act(async () => {
      slow.resolve(metrics);
      await Promise.resolve();
    });
    expect(screen.getByRole('region', { name: 'Current source metrics' })).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(4999);
    });
    expect(api.listMetrics).toHaveBeenCalledTimes(1);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    await settle();
    expect(api.listMetrics).toHaveBeenCalledTimes(2);
  });

  it('does not let a canceled old source failure replace a loaded new source', async () => {
    setup();
    const api = fixture();
    const old = deferred<ProductMetricHistoryResult>();
    api.listMetricHistory.mockImplementationOnce(() => old.promise);
    const view = render(mount(api));
    await settle();
    const signal = api.listMetricHistory.mock.calls[0]?.[0]?.signal;
    view.rerender(mount(api, 'source_docker'));
    await settle();
    expect(signal?.aborted).toBe(true);
    expect(screen.getByRole('region', { name: 'Current source metrics' })).not.toBeNull();
    await act(async () => {
      old.reject(new Error('Late canceled read failed.'));
      await Promise.resolve();
    });
    expect(screen.queryByText('Data could not load.')).toBeNull();
    expect(screen.getByRole('region', { name: 'Current source metrics' })).not.toBeNull();
  });

  it('changes a correlated time range with one batch and the same trusted fixture clock', async () => {
    setup();
    const api = fixture();
    render(mount(api));
    await settle();
    fireEvent.change(screen.getByRole('combobox', { name: 'Time range' }), {
      target: { value: '360' },
    });
    await settle();
    expect(api.listMetricHistory).toHaveBeenCalledTimes(2);
    expect(api.listMetricHistory.mock.calls[1]?.[0]).toMatchObject({
      from: '2026-08-16T02:00:00.001Z',
      to: '2026-08-16T08:00:00.001Z',
    });
    expect(api.listEvents.mock.calls[1]?.[0]).toMatchObject({
      from: '2026-08-16T02:00:00.001Z',
      to: '2026-08-16T08:00:00.001Z',
    });
  });
});
