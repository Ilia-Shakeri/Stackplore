import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { ProductEventsPage } from './ExplorationEventsPage';
import { ProductLogsPage } from './ExplorationLogsPage';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';

const at = '2026-08-16T08:00:00Z';
const resourceId = `r1_${'A'.repeat(43)}`;
const page = {
  availability: 'available' as const,
  completeness: 'complete' as const,
  observed_at: at,
  items: [],
  page: { limit: 100, has_more: false as const },
};

class Stream extends EventTarget {
  static instances: Stream[] = [];
  close = vi.fn();
  constructor() {
    super();
    Stream.instances.push(this);
  }
}

function baseApi(): ProductApi {
  return {
    ...createProductApi({ request: () => Promise.reject(new Error('Unexpected request.')) }),
    listSources: vi.fn().mockResolvedValue([]),
    listEnvironments: vi.fn().mockResolvedValue(page),
  };
}

afterEach(() => {
  Stream.instances = [];
  vi.unstubAllGlobals();
  window.history.replaceState(null, '', '/');
});

describe('stream refetch with fixed observation windows', () => {
  it('refetches an absolute event window after overflow reset and normal recovery', async () => {
    vi.stubGlobal('EventSource', Stream);
    const listEvents = vi.fn<ProductApi['listEvents']>().mockResolvedValue(page);
    render(
      <LocaleProvider defaultLocale="en">
        <ProductProvider
          api={{ ...baseApi(), listEvents }}
          sourceId="source_demo"
          onSourceChange={() => {}}
          viewSearch="from=2026-08-16T07%3A00%3A00Z&to=2026-08-16T08%3A00%3A00Z&environment_id=env_demo&event_q=gateway&severity=warning"
        >
          <ProductEventsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    await waitFor(() => {
      expect(listEvents).toHaveBeenCalledTimes(1);
    });
    for (const event of ['stream.reset.v1', 'events.snapshot.v1']) {
      const before = listEvents.mock.calls.length;
      act(() => {
        Stream.instances[0]?.dispatchEvent(
          new MessageEvent(event, { data: '{"reason":"snapshot_overflow"}' }),
        );
      });
      await waitFor(() => {
        expect(listEvents).toHaveBeenCalledTimes(before + 1);
      });
      expect(listEvents.mock.calls.at(-1)?.[0]).toMatchObject({
        from: '2026-08-16T07:00:00Z',
        to: '2026-08-16T08:00:00Z',
        environmentId: 'env_demo',
        sourceId: 'source_demo',
        search: 'gateway',
        severity: 'warning',
      });
    }
  });

  it.each(['poll', 'overflow'] as const)(
    'refetches fixed-clock synthetic logs after %s and retains pause cleanup',
    async (trigger) => {
      vi.stubGlobal('EventSource', Stream);
      let poll: (() => void) | undefined;
      const realTimer = window.setTimeout.bind(window);
      vi.spyOn(window, 'setTimeout').mockImplementation((handler, delay, ...args) => {
        if (delay === 5000 && typeof handler === 'function')
          poll = () => {
            handler(...args);
          };
        return realTimer(handler, delay, ...args) as unknown as ReturnType<
          typeof window.setTimeout
        >;
      });
      const listLogs = vi.fn<ProductApi['listLogs']>().mockResolvedValue(page);
      const api: ProductApi = {
        ...baseApi(),
        listLogs,
        listSources: vi.fn().mockResolvedValue([
          {
            id: 'source_demo',
            provider: 'demo',
            name: 'Demo',
            synthetic: true,
            observed_at: at,
            availability: 'available',
            completeness: 'complete',
            resource_count: 0,
            health: { healthy: 0, degraded: 0, unhealthy: 0, unknown: 0 },
          },
        ]),
      };
      const user = userEvent.setup();
      const { unmount } = render(
        <LocaleProvider defaultLocale="en">
          <ProductProvider
            api={api}
            sourceId="source_demo"
            onSourceChange={() => {}}
            viewSearch={`resource_id=${resourceId}&container=worker&tail=live`}
          >
            <ProductLogsPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      await waitFor(() => {
        expect(listLogs.mock.calls.at(-1)?.[0]?.to).toBe('2026-08-16T08:00:00.001Z');
        expect(poll).toBeTypeOf('function');
      });
      const before = listLogs.mock.calls.length;
      act(() => {
        if (trigger === 'poll') poll?.();
        else
          Stream.instances[0]?.dispatchEvent(
            new MessageEvent('stream.reset.v1', { data: '{"reason":"snapshot_overflow"}' }),
          );
      });
      await waitFor(() => {
        expect(listLogs).toHaveBeenCalledTimes(before + 1);
      });
      expect(listLogs.mock.calls.at(-1)?.[0]).toMatchObject({
        from: '2026-08-16T07:00:00.001Z',
        to: '2026-08-16T08:00:00.001Z',
        resourceId,
        container: 'worker',
        sourceId: 'source_demo',
      });
      await user.click(screen.getByRole('button', { name: 'Pause live tail' }));
      expect(Stream.instances[0]?.close).toHaveBeenCalledOnce();
      await user.click(screen.getByRole('button', { name: 'Resume live tail' }));
      const resumed = Stream.instances.at(-1);
      const resumedBefore = listLogs.mock.calls.length;
      act(() => {
        resumed?.dispatchEvent(
          new MessageEvent('stream.reset.v1', { data: '{"reason":"snapshot_overflow"}' }),
        );
      });
      await waitFor(() => {
        expect(listLogs).toHaveBeenCalledTimes(resumedBefore + 1);
      });
      unmount();
      expect(resumed?.close).toHaveBeenCalledOnce();
    },
  );
});
