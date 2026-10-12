import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { App } from '../../app/App';
import { createAppRouter, sanitizeNextPath } from '../../app/router';
import {
  createAuthenticatedAuthStore,
  createResolvedAuthStore,
  createTestSession,
  testProof,
} from '../../test/auth';

function renderAuthRoute(path: string, store: ReturnType<typeof createResolvedAuthStore>) {
  const history = createMemoryHistory({ initialEntries: [path] });
  const router = createAppRouter(history, store);
  return render(<App router={router} />);
}

describe('auth routes and forms', () => {
  it('routes first-run state to setup and shows proof only when required', async () => {
    const store = createResolvedAuthStore({
      bootstrapProofRequired: true,
      status: 'setup_required',
    });
    renderAuthRoute('/overview', store);

    expect(await screen.findByRole('heading', { level: 1, name: 'Setup' })).not.toBeNull();
    const proof = screen.getByLabelText(/Operator bootstrap proof/u);
    expect(proof.getAttribute('minlength')).toBe('43');
    expect(proof.getAttribute('maxlength')).toBe('43');
    expect(proof.getAttribute('pattern')).toBe('[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]');
    expect(screen.queryByRole('complementary')).toBeNull();
  });

  it('routes a signed-out deep link to login without mounting the app shell', async () => {
    const store = createResolvedAuthStore({ status: 'unauthenticated' });
    renderAuthRoute('/topology', store);

    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).not.toBeNull();
    expect(screen.queryByRole('complementary')).toBeNull();
  });

  it('returns to a safe internal page after sign in', async () => {
    const user = userEvent.setup();
    const login = vi.fn().mockResolvedValue(createTestSession());
    const store = createResolvedAuthStore({ status: 'unauthenticated' }, { login });
    renderAuthRoute('/login?next=%2Ftopology', store);

    await user.type(await screen.findByLabelText(/Login name/u), 'operator');
    await user.type(screen.getByLabelText(/^Password/u), 'long-password');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'Topology' })).not.toBeNull();
    expect(login).toHaveBeenCalledWith({ loginName: 'operator', password: 'long-password' });
  });

  it('uses UTF-8 byte bounds instead of a character minimum for passwords', async () => {
    const user = userEvent.setup();
    const login = vi.fn().mockResolvedValue(createTestSession());
    const store = createResolvedAuthStore({ status: 'unauthenticated' }, { login });
    renderAuthRoute('/login', store);

    await user.type(await screen.findByLabelText(/Login name/u), 'operator');
    await user.type(screen.getByLabelText(/^Password/u), 'رمزعبور');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(login).toHaveBeenCalledWith({ loginName: 'operator', password: 'رمزعبور' });
  });

  it('blocks mismatched setup passwords before a request', async () => {
    const user = userEvent.setup();
    const setup = vi.fn().mockResolvedValue(createTestSession());
    const store = createResolvedAuthStore(
      { bootstrapProofRequired: false, status: 'setup_required' },
      { setup },
    );
    renderAuthRoute('/setup', store);

    await user.type(await screen.findByLabelText(/Display name/u), 'Operations');
    await user.type(screen.getByLabelText(/Login name/u), 'operator');
    await user.type(screen.getByLabelText(/^Password/u), 'long-password');
    await user.type(screen.getByLabelText(/Confirm password/u), 'other-password');
    await user.click(screen.getByRole('button', { name: 'Create administrator' }));

    expect(await screen.findByText('The password confirmation does not match.')).not.toBeNull();
    expect(setup).not.toHaveBeenCalled();
  });

  it('cuts administrator routes and links for a viewer', async () => {
    const store = createAuthenticatedAuthStore('viewer');
    const history = createMemoryHistory({ initialEntries: ['/audit'] });
    const router = createAppRouter(history, store);
    render(<App router={router} />);

    expect(await screen.findByRole('heading', { level: 1, name: 'Overview' })).not.toBeNull();
    const sidebar = screen.getByRole('complementary');
    expect(within(sidebar).queryByRole('link', { name: 'Audit' })).toBeNull();
    expect(within(sidebar).getByRole('link', { name: 'Settings' }).getAttribute('href')).toBe(
      '/settings/preferences',
    );
  });

  it('signs out through the user menu and leaves no app shell', async () => {
    const user = userEvent.setup();
    const logout = vi.fn().mockResolvedValue(undefined);
    const store = createAuthenticatedAuthStore('administrator', { logout });
    const history = createMemoryHistory({ initialEntries: ['/overview'] });
    const router = createAppRouter(history, store);
    render(<App router={router} />);

    await user.click(await screen.findByRole('button', { name: 'Open user menu' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Sign out' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).not.toBeNull();
    expect(logout).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('complementary')).toBeNull();
  });

  it('changes the current password and rotates the in-memory session', async () => {
    const user = userEvent.setup();
    const recentProof = `${'B'.repeat(42)}A`;
    const createRecentAuthGrant = vi.fn().mockResolvedValue(recentProof);
    const changeOwnPassword = vi.fn().mockResolvedValue(createTestSession());
    const store = createAuthenticatedAuthStore('viewer', {
      changeOwnPassword,
      createRecentAuthGrant,
    });
    const history = createMemoryHistory({ initialEntries: ['/overview'] });
    const router = createAppRouter(history, store);
    render(<App router={router} />);

    await user.click(await screen.findByRole('button', { name: 'Open user menu' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Change password' }));
    const dialog = await screen.findByRole('dialog', { name: 'Change password' });
    await user.type(within(dialog).getByLabelText(/Current password/u), 'current-password');
    await user.type(within(dialog).getByLabelText(/^New password/u), 'replacement-password');
    await user.type(within(dialog).getByLabelText(/Confirm password/u), 'replacement-password');
    await user.click(within(dialog).getByRole('button', { name: 'Change password' }));

    expect(await screen.findByText('Password changed')).not.toBeNull();
    expect(createRecentAuthGrant).toHaveBeenCalledWith(
      'current-password',
      'account.password_change',
      testProof,
    );
    expect(changeOwnPassword).toHaveBeenCalledWith(
      'replacement-password',
      expect.objectContaining({ recentAuthToken: recentProof }),
    );
  });

  it('clears the password after a rejected login', async () => {
    const user = userEvent.setup();
    const login = vi.fn().mockRejectedValue(new Error('offline'));
    const store = createResolvedAuthStore({ status: 'unauthenticated' }, { login });
    renderAuthRoute('/login', store);

    await user.type(await screen.findByLabelText(/Login name/u), 'operator');
    const password = screen.getByLabelText(/^Password/u);
    await user.type(password, 'long-password');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText(/request could not be completed/u)).not.toBeNull();
    expect((password as HTMLInputElement).value).toBe('');
  });

  it('renders the Persian login form in RTL', async () => {
    globalThis.localStorage.setItem('stackplore.locale', 'fa');
    const store = createResolvedAuthStore({ status: 'unauthenticated' });
    renderAuthRoute('/login', store);

    expect(await screen.findByRole('heading', { level: 1, name: 'ورود' })).not.toBeNull();
    expect(screen.getByLabelText(/نام ورود/u).getAttribute('dir')).toBe('ltr');
    expect(document.documentElement.dir).toBe('rtl');
  });

  it('accepts only bounded local return paths', () => {
    expect(sanitizeNextPath('/resources/source_demo/demo/container/item')).toBe(
      '/resources/source_demo/demo/container/item',
    );
    for (const value of ['https://outside.example', '//outside.example', '/\\outside', '/login']) {
      expect(sanitizeNextPath(value)).toBeUndefined();
    }
  });
});
