import { createMemoryHistory } from '@tanstack/react-router';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from '../App';
import { createAppRouter } from '../router';
import { createAuthenticatedAuthStore } from '../../test/auth';

function renderPalette(role: 'administrator' | 'viewer' = 'administrator') {
  const history = createMemoryHistory({ initialEntries: ['/about?source=source_demo'] });
  const router = createAppRouter(history, createAuthenticatedAuthStore(role));
  render(<App router={router} />);
  return router;
}

describe('command palette', () => {
  it.each(['en', 'fa'])(
    'filters and opens fixed routes with source preserved in %s',
    async (locale) => {
      localStorage.setItem('stackplore.locale', locale);
      const user = userEvent.setup();
      const router = renderPalette();
      const trigger = await screen.findByRole('button', {
        name: locale === 'fa' ? 'بازکردن پالت فرمان' : 'Open command palette',
      });
      await user.click(trigger);
      const dialog = await screen.findByRole('dialog', {
        name: locale === 'fa' ? 'رفتن به صفحه' : 'Go to a page',
      });
      const search = within(dialog).getByRole('combobox');
      expect(document.activeElement).toBe(search);
      await user.type(search, '/overview');
      expect(within(dialog).getAllByRole('option')).toHaveLength(1);
      await user.keyboard('{Enter}');
      await waitFor(() => {
        expect(router.state.location.pathname).toBe('/overview');
      });
      expect(router.state.location.search.source).toBe('source_demo');
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).toBeNull();
      });
      await waitFor(() => {
        expect(document.activeElement).toBe(screen.getByRole('main'));
      });
    },
  );

  it('supports both shortcuts, arrow selection, Escape and return to the invoking field', async () => {
    const user = userEvent.setup();
    const router = renderPalette();
    const invokingField = await screen.findByRole('searchbox', { name: 'Search resources' });
    await user.click(invokingField);
    fireEvent.keyDown(invokingField, { key: 'k', ctrlKey: true });
    const dialog = await screen.findByRole('dialog', { name: 'Go to a page' });
    const search = within(dialog).getByRole('combobox');
    await user.type(search, 'not-a-page');
    expect(within(dialog).getByRole('status').textContent).toBe('No matching page.');
    await user.keyboard('{Enter}');
    expect(router.state.location.pathname).toBe('/about');
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(document.activeElement).toBe(invokingField);
    });
    fireEvent.keyDown(invokingField, { key: 'k', metaKey: true });
    const reopened = await screen.findByRole('dialog', { name: 'Go to a page' });
    expect(within(reopened).getByRole<HTMLInputElement>('combobox').value).toBe('');
    await user.keyboard('{ArrowDown}{Enter}');
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/topology');
    });
  });

  it('matches viewer navigation authority and does not invent actions from a query', async () => {
    const user = userEvent.setup();
    const router = renderPalette('viewer');
    await user.click(await screen.findByRole('button', { name: 'Open command palette' }));
    const dialog = await screen.findByRole('dialog', { name: 'Go to a page' });
    const search = within(dialog).getByRole('combobox');
    expect(within(dialog).queryByRole('option', { name: /Audit/ })).toBeNull();
    await user.type(search, 'https://other.invalid/');
    expect(within(dialog).queryAllByRole('option')).toHaveLength(0);
    await user.keyboard('{Enter}');
    expect(router.state.location.pathname).toBe('/about');
    await user.clear(search);
    await user.type(search, 'Settings');
    await user.keyboard('{Enter}');
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/settings/preferences');
    });
  });

  it('leaves an existing modal in control of its shortcut and ignores composition', async () => {
    renderPalette();
    await screen.findByRole('button', { name: 'Open command palette' });
    const other = document.createElement('div');
    other.setAttribute('role', 'dialog');
    other.setAttribute('data-state', 'open');
    document.body.append(other);
    try {
      fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
      expect(screen.queryByRole('dialog', { name: 'Go to a page' })).toBeNull();
    } finally {
      other.remove();
    }
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true, isComposing: true });
    expect(screen.queryByRole('dialog', { name: 'Go to a page' })).toBeNull();
  });
});
