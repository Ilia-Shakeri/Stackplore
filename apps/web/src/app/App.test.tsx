import { createMemoryHistory } from '@tanstack/react-router';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from './App';
import { createAppRouter } from './router';
import { createAuthenticatedAuthStore } from '../test/auth';
import { createTestUsersApi } from '../test/users';

function renderRoute(path: string) {
  const history = createMemoryHistory({ initialEntries: [path] });
  const router = createAppRouter(
    history,
    createAuthenticatedAuthStore(),
    createTestUsersApi({
      listUsers: () => Promise.resolve({ items: [] }),
    }),
  );
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

  it.each([
    ['/overview', 'نمای کلی'],
    ['/topology', 'توپولوژی'],
    ['/resources', 'منابع'],
    ['/logs', 'گزارش‌ها'],
    ['/events', 'رویدادها'],
    ['/alerts', 'هشدارها'],
    ['/audit', 'ممیزی'],
    ['/settings', 'تنظیمات'],
    ['/settings/connections', 'اتصال‌ها'],
    ['/settings/users', 'کاربران'],
    ['/settings/notifications', 'اعلان‌ها'],
    ['/settings/preferences', 'ترجیحات'],
    ['/about', 'درباره'],
    ['/not-found', 'صفحه پیدا نشد'],
  ])('renders Persian shell copy for %s', async (path, heading) => {
    globalThis.localStorage.setItem('stackplore.locale', 'fa');
    renderRoute(path);

    expect(await screen.findByRole('heading', { level: 1, name: heading })).not.toBeNull();
    expect(document.documentElement.lang).toBe('fa');
    expect(document.documentElement.dir).toBe('rtl');
  });

  it('switches shell language without a reload and preserves theme state', async () => {
    renderRoute('/overview');

    const language = await screen.findByRole('combobox', { name: 'Language' });
    fireEvent.change(language, { target: { value: 'fa' } });

    expect(await screen.findByRole('heading', { level: 1, name: 'نمای کلی' })).not.toBeNull();
    expect(screen.getByRole('link', { name: 'توپولوژی' })).not.toBeNull();
    expect(screen.getByRole('combobox', { name: 'زبان' })).not.toBeNull();
    expect(document.documentElement.lang).toBe('fa');
    expect(document.documentElement.dir).toBe('rtl');
    expect(document.title).toBe('نمای کلی | Stackplore');
    expect(globalThis.localStorage.getItem('stackplore.locale')).toBe('fa');

    fireEvent.change(screen.getByRole('combobox', { name: 'پوسته' }), {
      target: { value: 'dark' },
    });

    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(globalThis.localStorage.getItem('stackplore.locale')).toBe('fa');
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

  it('keeps technical resource identity exact and isolated in Persian', async () => {
    globalThis.localStorage.setItem('stackplore.locale', 'fa');
    renderRoute('/resources/demo/container/service%2Fedge');

    expect(await screen.findByRole('heading', { level: 1, name: 'جزئیات منبع' })).not.toBeNull();

    for (const value of ['demo', 'container', 'service/edge']) {
      const technical = screen.getByText(value);
      expect(technical.getAttribute('dir')).toBe('ltr');
      expect(technical.hasAttribute('data-ui-technical')).toBe(true);
    }
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

  it('uses Persian names in mobile navigation and keeps route focus behavior', async () => {
    const user = userEvent.setup();
    globalThis.localStorage.setItem('stackplore.locale', 'fa');
    renderRoute('/overview');

    await user.click(await screen.findByRole('button', { name: 'باز کردن ناوبری' }));
    const drawer = await screen.findByRole('dialog', { name: 'ناوبری' });

    await user.click(within(drawer).getByRole('link', { name: 'توپولوژی' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'توپولوژی' })).not.toBeNull();
    expect(screen.queryByRole('dialog', { name: 'ناوبری' })).toBeNull();
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByRole('main'));
    });
  });
});
