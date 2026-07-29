import { createMemoryHistory } from '@tanstack/react-router';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from './App';
import { createAppRouter } from './router';

Object.defineProperty(globalThis, 'scrollTo', {
  configurable: true,
  value: () => undefined,
  writable: true,
});

function renderRoute(path: string) {
  const history = createMemoryHistory({ initialEntries: [path] });
  const router = createAppRouter(history);
  return render(<App router={router} />);
}

describe('application shell', () => {
  it('redirects the root to an accessible overview shell', async () => {
    renderRoute('/');

    const main = await screen.findByRole('main');
    const heading = await screen.findByRole('heading', { level: 1, name: 'Overview' });
    const activeLink = screen.getByRole('link', { name: 'Overview' });
    const theme = screen.getByRole('combobox', { name: 'Theme' });

    expect(main.contains(heading)).toBe(true);
    expect(screen.getAllByRole('main')).toHaveLength(1);
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(activeLink.getAttribute('aria-current')).toBe('page');
    expect(document.title).toBe('Overview | Stackplore');

    fireEvent.change(theme, { target: { value: 'dark' } });

    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(globalThis.localStorage.getItem('stackplore.theme')).toBe('dark');
  });

  it.each([
    ['/setup', 'Setup'],
    ['/login', 'Sign in'],
    ['/overview', 'Overview'],
    ['/topology', 'Topology'],
    ['/resources', 'Resources'],
    ['/logs', 'Logs'],
    ['/events', 'Events'],
    ['/alerts', 'Alerts'],
    ['/audit', 'Audit'],
    ['/settings', 'Settings'],
    ['/settings/connections', 'Connections'],
    ['/settings/users', 'Users'],
    ['/settings/notifications', 'Notifications'],
    ['/settings/preferences', 'Preferences'],
    ['/about', 'About'],
    ['/not-found', 'Page not found'],
  ])('renders %s at its direct URL', async (path, heading) => {
    renderRoute(path);

    expect(await screen.findByRole('heading', { level: 1, name: heading })).not.toBeNull();
  });

  it('keeps resource route parameters exact', async () => {
    renderRoute('/resources/demo/container/service%2Fedge');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Resource detail' }),
    ).not.toBeNull();
    expect(screen.getByText('demo')).not.toBeNull();
    expect(screen.getByText('container')).not.toBeNull();
    expect(screen.getByText('service/edge')).not.toBeNull();
  });

  it('routes an unknown URL to the shell not-found view', async () => {
    renderRoute('/missing-route');

    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).not.toBeNull();
    expect(screen.getAllByRole('main')).toHaveLength(1);
  });

  it('opens mobile navigation and restores trigger focus after Escape', async () => {
    const user = userEvent.setup();
    renderRoute('/overview');

    const trigger = await screen.findByRole('button', { name: 'Open navigation' });
    trigger.focus();
    await user.click(trigger);

    expect(await screen.findByRole('dialog', { name: 'Navigation' })).not.toBeNull();

    await user.keyboard('{Escape}');

    expect(screen.queryByRole('dialog', { name: 'Navigation' })).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it('keeps collapsed sidebar links named', async () => {
    const user = userEvent.setup();
    renderRoute('/overview');

    const sidebar = await screen.findByRole('complementary');
    const collapse = within(sidebar).getByRole('button', { name: 'Collapse sidebar' });

    await user.click(collapse);

    const topology = within(sidebar).getByRole('link', { name: 'Topology' });
    const label = within(topology).getByText('Topology');

    expect(collapse.getAttribute('aria-expanded')).toBe('false');
    expect(label.classList.contains('ui-sr-only')).toBe(true);
  });

  it('closes mobile navigation and focuses main content after a route change', async () => {
    const user = userEvent.setup();
    renderRoute('/overview');

    await user.click(await screen.findByRole('button', { name: 'Open navigation' }));
    const drawer = await screen.findByRole('dialog', { name: 'Navigation' });

    await user.click(within(drawer).getByRole('link', { name: 'Topology' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'Topology' })).not.toBeNull();
    expect(screen.queryByRole('dialog', { name: 'Navigation' })).toBeNull();
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByRole('main'));
    });
  });
});
