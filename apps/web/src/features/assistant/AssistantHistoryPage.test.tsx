import type { AssistantHistoryEntry, AssistantHistoryResponse } from '@stackplore/api-client';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import { AssistantHistoryPage } from './AssistantHistoryPage';
import { AssistantProvider } from './assistant-context';
import { controlledConversationApi, scope } from './conversation.test-fixtures';
import { AuthProvider } from '../auth/auth-context';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { LocaleProvider, translateText } from '../../i18n';

const record = (source = scope.sourceId): AssistantHistoryEntry => ({
  id: `ar_${'b'.repeat(64)}`,
  source_id: source,
  resource_id: scope.resourceId,
  goal: 'investigate_incident',
  state: 'completed',
  created_at: '2026-11-08T06:00:00Z',
  updated_at: '2026-11-08T06:01:00Z',
  range: { from: '2026-11-08T05:45:00Z', to: '2026-11-08T06:00:00Z' },
  tool_calls: 7,
  inference_calls: 1,
  usage_uncertain: false,
});
function show(locale: 'en' | 'fa' = 'en', controlled = controlledConversationApi()) {
  const rendered = render(
    <LocaleProvider defaultLocale={locale}>
      <AuthProvider store={createAuthenticatedAuthStore('viewer')}>
        <AssistantProvider api={controlled.api}>
          <AssistantHistoryPage />
        </AssistantProvider>
      </AuthProvider>
    </LocaleProvider>,
  );
  return { ...rendered, ...controlled };
}
afterEach(() => {
  window.history.replaceState(null, '', '/');
});

describe('separate private history page', () => {
  it.each(['en', 'fa'] as const)(
    'pages with the keyboard and replaces rows without replay in %s',
    async (locale) => {
      const user = userEvent.setup();
      const controlled = controlledConversationApi();
      const cursor = `${'a'.repeat(50)}.${'b'.repeat(43)}`;
      controlled.api.history
        .mockResolvedValueOnce({
          entries: Array.from({ length: 10 }, (_, index) => ({
            ...record(),
            id: `ar_${(10 - index).toString(16).padStart(64, '0')}`,
          })),
          next_cursor: cursor,
        })
        .mockResolvedValueOnce({ entries: [record('source_next')], next_cursor: null });
      const { api } = show(locale, controlled);
      const next = await screen.findByRole('button', {
        name: translateText(locale, 'assistant.history.next'),
      });
      await waitFor(() => {
        expect((next as HTMLButtonElement).disabled).toBe(false);
      });
      expect(screen.getAllByRole('listitem')).toHaveLength(10);
      next.focus();
      await user.keyboard('{Enter}');
      await screen.findByText('source_next');
      expect(screen.getAllByRole('listitem')).toHaveLength(1);
      expect(screen.queryByText(scope.sourceId)).toBeNull();
      expect(api.history).toHaveBeenLastCalledWith(
        api.getOwner(),
        { state: 'completed', limit: 10, cursor },
        expect.any(AbortSignal),
      );
      expect(document.activeElement).toBe(
        screen.getByRole('heading', { name: translateText(locale, 'assistant.history.records') }),
      );
      expect((next as HTMLButtonElement).disabled).toBe(true);
      expect(
        screen
          .getByRole('link', { name: translateText(locale, 'assistant.history.openResource') })
          .getAttribute('href'),
      ).toBe(`/assistant?source=source_next&resource_id=${scope.resourceId}`);
      expect(window.location.href).not.toContain(cursor);
      expect(api.admit).not.toHaveBeenCalled();
      expect(api.report).not.toHaveBeenCalled();
      expect(api.status).not.toHaveBeenCalled();
    },
  );

  it('hides old-owner rows and ignores a late aborted response', async () => {
    const controlled = controlledConversationApi();
    let complete: (value: AssistantHistoryResponse) => void = () => undefined;
    controlled.api.history
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            complete = resolve;
          }),
      )
      .mockResolvedValueOnce({ entries: [record('new_owner')], next_cursor: null });
    const { api } = show('en', controlled);
    await waitFor(() => {
      expect(api.history).toHaveBeenCalledTimes(1);
    });
    const oldSignal = api.history.mock.calls[0]?.[2];
    act(() => {
      controlled.setOwner(Symbol());
    });
    await screen.findByText('new_owner');
    await act(async () => {
      complete({ entries: [record('old_owner')], next_cursor: null });
      await Promise.resolve();
    });
    expect(oldSignal?.aborted).toBe(true);
    expect(screen.queryByText('old_owner')).toBeNull();
    act(() => {
      controlled.setOwner(null);
    });
    expect(screen.queryByText('new_owner')).toBeNull();
  });

  it('uses a URL status filter and gives an explicit first-page recovery', async () => {
    const { api } = show();
    await screen.findByText('No retained runs with this status.');
    api.history.mockRejectedValueOnce(new Error('Private internal detail'));
    fireEvent.change(screen.getByRole('combobox', { name: 'Run status' }), {
      target: { value: 'failed' },
    });
    await screen.findByRole('alert');
    expect(new URLSearchParams(window.location.search).get('history_state')).toBe('failed');
    expect(screen.queryByText('Private internal detail')).toBeNull();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Refresh from first page' }));
    await screen.findByText('No retained runs with this status.');
    expect(api.history).toHaveBeenLastCalledWith(
      api.getOwner(),
      { state: 'failed', limit: 10 },
      expect.any(AbortSignal),
    );
  });

  it('clears rows on page exit and starts a fresh read on return', async () => {
    const controlled = controlledConversationApi();
    controlled.api.history.mockResolvedValueOnce({ entries: [record()], next_cursor: null });
    const { api } = show('en', controlled);
    await screen.findByText(scope.sourceId);
    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });
    expect(screen.queryByText(scope.sourceId)).toBeNull();
    expect(api.history.mock.calls[0]?.[2]?.aborted).toBe(true);
    act(() => {
      window.dispatchEvent(new Event('pageshow'));
    });
    await screen.findByText('No retained runs with this status.');
    expect(api.history).toHaveBeenCalledTimes(2);
  });
});
