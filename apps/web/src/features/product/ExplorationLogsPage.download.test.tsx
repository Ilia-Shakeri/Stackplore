import { ApiProblemError } from '@stackplore/api-client';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { ProductLogsPage } from './ExplorationLogsPage';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';

const resource = `r1_${'A'.repeat(43)}`;
const from = '2026-08-16T07:00:00Z';
const to = '2026-08-16T08:00:00Z';
const selection = `?source=source_demo&resource_id=${resource}&container=api&previous=true&from=${from}&to=${to}&tail=paused&log_q=worker&severity=warning`;
const createUrl = vi.fn().mockReturnValue('blob:filtered-logs');
const revokeUrl = vi.fn();
const apiFor = (downloadLogs: ProductApi['downloadLogs']): ProductApi => ({
  ...createProductApi({ request: () => Promise.reject(new Error('Unexpected test request.')) }),
  downloadLogs,
  listSources: vi.fn().mockResolvedValue([]),
  listLogs: vi.fn().mockResolvedValue({
    availability: 'available',
    completeness: 'complete',
    observed_at: to,
    items: [],
    page: { has_more: false, limit: 100 },
  }),
});
const setupUrls = () => {
  class DownloadURL extends URL {
    static override createObjectURL = createUrl;
    static override revokeObjectURL = revokeUrl;
  }
  vi.stubGlobal('URL', DownloadURL);
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
};
afterEach(() => {
  window.history.replaceState(null, '', '/');
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('filtered log download', () => {
  it('downloads the server selection even when no rows are loaded and states limits in Persian', async () => {
    setupUrls();
    window.history.replaceState(null, '', `/logs${selection}`);
    const download = vi
      .fn<ProductApi['downloadLogs']>()
      .mockResolvedValue({ text: 'bounded server text', limited: true });
    render(
      <LocaleProvider defaultLocale="fa">
        <ProductProvider api={apiFor(download)}>
          <ProductLogsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    const button = await screen.findByRole('button', { name: 'دریافت گزارش‌های فیلترشده' });
    await userEvent.setup().click(button);
    expect(download.mock.calls[0]?.[0]).toMatchObject({
      sourceId: 'source_demo',
      resourceId: resource,
      container: 'api',
      previous: true,
      from,
      to,
      search: 'worker',
      severity: 'warning',
    });
    expect(createUrl).toHaveBeenCalledOnce();
    expect(
      await screen.findByText(
        'فایل گزارش آماده شد. سقف ایمنی رسید؛ برای جزئیات بیشتر بازهٔ زمان را کوچک‌تر کنید.',
      ),
    ).not.toBeNull();
    await waitFor(() => {
      expect(revokeUrl).toHaveBeenCalledWith('blob:filtered-logs');
    });
  });

  it('aborts a changed selection and ignores a late download result', async () => {
    setupUrls();
    let finish!: (value: Awaited<ReturnType<ProductApi['downloadLogs']>>) => void;
    const download = vi.fn<ProductApi['downloadLogs']>().mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const api = apiFor(download);
    const content = (viewSearch: string) => (
      <LocaleProvider defaultLocale="en">
        <ProductProvider api={api} viewSearch={viewSearch}>
          <ProductLogsPage />
        </ProductProvider>
      </LocaleProvider>
    );
    const { rerender, unmount } = render(content(selection));
    await userEvent
      .setup()
      .click(await screen.findByRole('button', { name: 'Download filtered logs' }));
    const signal = download.mock.calls[0]?.[0]?.signal;
    expect(signal?.aborted).toBe(false);
    rerender(content(selection.replace('worker', 'changed')));
    await waitFor(() => {
      expect(signal?.aborted).toBe(true);
    });
    await act(async () => {
      finish({ text: 'late old selection', limited: false });
      await Promise.resolve();
    });
    expect(createUrl).not.toHaveBeenCalled();
    unmount();
  });

  it.each([false, true])(
    'saves no file for incomplete or policy-denied downloads (%s)',
    async (denied) => {
      setupUrls();
      const problem = new ApiProblemError({
        code: 'permission_denied',
        title: 'Permission denied',
        type: 'urn:stackplore:problem:permission_denied',
        status: 403,
        request_id: 'req_log_download_test',
      });
      const download = vi
        .fn<ProductApi['downloadLogs']>()
        .mockRejectedValue(denied ? problem : new Error('Incomplete download.'));
      render(
        <LocaleProvider defaultLocale="en">
          <ProductProvider api={apiFor(download)} viewSearch={selection}>
            <ProductLogsPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      await userEvent
        .setup()
        .click(await screen.findByRole('button', { name: 'Download filtered logs' }));
      expect(
        await screen.findByText(
          denied
            ? 'Log download is not allowed by the current account or capture policy.'
            : 'Log download failed or was incomplete. No file was saved. Try again.',
        ),
      ).not.toBeNull();
      expect(createUrl).not.toHaveBeenCalled();
    },
  );
});
