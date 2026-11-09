import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { RunbooksPage } from './RunbooksPage';
import { AssistantProvider } from './assistant-context';
import { controlledConversationApi } from './conversation.test-fixtures';
import { AuthProvider } from '../auth/auth-context';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { LocaleProvider, translateText } from '../../i18n';

const reference = {
  corpus_id: 'ops',
  version: 1,
  digest: 'a'.repeat(64),
  document_id: 'help',
  locale: 'en' as const,
  generation: 0,
};
function show(
  locale: 'en' | 'fa' = 'en',
  role: 'viewer' | 'administrator' = 'viewer',
  controlled = controlledConversationApi(),
) {
  render(
    <LocaleProvider defaultLocale={locale}>
      <AuthProvider store={createAuthenticatedAuthStore(role)}>
        <AssistantProvider api={controlled.api}>
          <RunbooksPage />
        </AssistantProvider>
      </AuthProvider>
    </LocaleProvider>,
  );
  return controlled;
}
describe('separate reviewed documents page', () => {
  it('removes private prose before the pagehide event finishes', async () => {
    const controlled = controlledConversationApi();
    controlled.api.runbookSearch.mockResolvedValue({
      hits: [{ reference, title: 'Private result', snippet: 'Safe snippet' }],
      next_cursor: '',
    });
    controlled.api.runbookDocument.mockResolvedValue({
      document: {
        id: 'help',
        locale: 'en',
        title: 'Private document',
        body: 'Clear this private prose synchronously',
      },
      provenance: null,
    });
    show('en', 'viewer', controlled);
    const user = userEvent.setup();
    await user.type(
      screen.getByRole('textbox', { name: 'Search in the current language' }),
      'service',
    );
    await user.click(screen.getByRole('button', { name: 'Search documents' }));
    await user.click(await screen.findByRole('button', { name: 'Private result' }));
    await screen.findByText('Clear this private prose synchronously');
    let visibleDuringExit = true;
    const inspect = () => {
      visibleDuringExit = document.body.textContent.includes(
        'Clear this private prose synchronously',
      );
    };
    window.addEventListener('pagehide', inspect, { once: true });
    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });
    expect(visibleDuringExit).toBe(false);
  });
  it('marks an old approval as needing review after restore', async () => {
    const controlled = controlledConversationApi();
    controlled.api.runbookCatalog.mockResolvedValue({
      generation: 1,
      version: 2,
      maximum_request_bytes: 262144,
      revisions: [
        {
          id: 'ops',
          version: 1,
          digest: 'a'.repeat(64),
          audience: 'all_users',
          approved: true,
          generation: 0,
          documents: 1,
          bytes: 100,
          provenance: null,
        },
      ],
    });
    show('en', 'administrator', controlled);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Review and approve' }));
    await user.click(screen.getByRole('button', { name: 'Load current catalog' }));
    await screen.findByText('Needs review after restore · 1');
    expect(controlled.api.runbookDocument).not.toHaveBeenCalled();
  });
  it.each(['en', 'fa'] as const)(
    'searches, opens inert text and clears it on owner change in %s',
    async (locale) => {
      const controlled = controlledConversationApi();
      controlled.api.runbookSearch.mockResolvedValue({
        hits: [
          { reference: { ...reference, locale }, title: 'Private result', snippet: 'Safe snippet' },
        ],
        next_cursor: '',
      });
      controlled.api.runbookDocument.mockResolvedValue({
        document: {
          id: 'help',
          locale,
          title: 'Opened document',
          body: '<script>private-inert-text</script>',
        },
        provenance: {
          kind: 'operator_authored',
          source: 'Reviewed fixture source',
          license: 'Fixture permission',
          use_reviewed: true, assistance_reviewed: false,
        },
      });
      show(locale, 'viewer', controlled);
      expect(
        screen.queryByRole('button', { name: translateText(locale, 'runbooks.manage') }),
      ).toBeNull();
      const user = userEvent.setup();
      await user.type(
        screen.getByRole('textbox', { name: translateText(locale, 'runbooks.query') }),
        'service',
      );
      await user.click(
        screen.getByRole('button', { name: translateText(locale, 'runbooks.search') }),
      );
      await user.click(await screen.findByRole('button', { name: 'Private result' }));
      await screen.findByText('<script>private-inert-text</script>');
      expect(screen.getByText('Reviewed fixture source').tagName).toBe('DD');
      expect(screen.getByRole('heading', { name: 'Opened document' })).toBe(document.activeElement);
      expect(document.querySelector('.runbooks-document script')).toBeNull();
      act(() => {
        controlled.setOwner(Symbol());
      });
      expect(screen.queryByText('<script>private-inert-text</script>')).toBeNull();
      expect(screen.queryByText('Private result')).toBeNull();
      expect(screen.queryByText('Reviewed fixture source')).toBeNull();
    },
  );
  it('discards preview when edited and fences a late result after leaving the page', async () => {
    const controlled = controlledConversationApi();
    controlled.api.runbookCatalog.mockResolvedValue({
      generation: 0,
      version: 1,
      revisions: [],
      maximum_request_bytes: 262144,
    });
    controlled.api.runbookPreview.mockResolvedValue({
      digest: 'a'.repeat(64),
      confirmation_token: 'A'.repeat(43),
      expires_at: new Date(Date.now() + 120000).toISOString(),
    });
    show('en', 'administrator', controlled);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Review and approve' }));
    await user.click(screen.getByRole('button', { name: 'Load current catalog' }));
    const input = await screen.findByRole('textbox', { name: 'Collection JSON' });
    const corpus = {
      id: 'ops',
      version: 1,
      audience: 'all_users',
      provenance: {
        kind: 'operator_authored',
        source: 'Synthetic fixture',
        license: 'Fixture author permission',
        use_reviewed: true, assistance_reviewed: false,
      },
      documents: [
        { id: 'help', locale: 'en', title: 'Private title', body: 'Private reviewed body' },
      ],
    };
    fireEvent.change(input, { target: { value: JSON.stringify(corpus) } });
    await user.click(screen.getByRole('button', { name: 'Review exact text' }));
    await screen.findByText('Private reviewed body');
    fireEvent.change(input, { target: { value: JSON.stringify({ ...corpus, version: 2 }) } });
    expect(screen.queryByText('Private reviewed body')).toBeNull();
    expect(screen.queryByRole('checkbox')).toBeNull();
    fireEvent.change(input, { target: { value: JSON.stringify(corpus) } });
    let finish:
      | ((value: { digest: string; confirmation_token: string; expires_at: string }) => void)
      | undefined;
    controlled.api.runbookPreview.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    await user.click(screen.getByRole('button', { name: 'Review exact text' }));
    await waitFor(() => {
      expect(finish).toBeDefined();
    });
    fireEvent(window, new Event('pagehide'));
    await act(async () => {
      await Promise.resolve();
      finish?.({
        digest: 'a'.repeat(64),
        confirmation_token: 'A'.repeat(43),
        expires_at: new Date(Date.now() + 120000).toISOString(),
      });
    });
    expect(screen.queryByText('Private reviewed body')).toBeNull();
    expect(controlled.api.runbookCommit).not.toHaveBeenCalled();
  });
});
