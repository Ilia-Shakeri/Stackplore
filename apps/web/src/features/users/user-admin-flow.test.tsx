import { ApiProblemError } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { App } from '../../app/App';
import { createAppRouter } from '../../app/router';
import { createAuthenticatedAuthStore, createTestUser } from '../../test/auth';
import { createTestUsersApi } from '../../test/users';

const administrator = createTestUser('administrator');
const viewer = createTestUser('viewer');
const finalPage = { items: [administrator, viewer] } as const;

const conflict = () =>
  new ApiProblemError({
    code: 'conflict',
    request_id: 'req_test_01',
    status: 409,
    title: 'Request conflict',
    type: 'urn:stackplore:problem:conflict',
  });

function renderUsers(
  users: ReturnType<typeof createTestUsersApi>,
  role: 'administrator' | 'viewer' = 'administrator',
) {
  const history = createMemoryHistory({ initialEntries: ['/settings/users'] });
  const router = createAppRouter(history, createAuthenticatedAuthStore(role), users);
  return render(<App router={router} />);
}

const userRow = async (name: string) => {
  const cell = await screen.findByText(name);
  const row = cell.closest('tr');
  if (row === null) {
    throw new Error('User row was not rendered.');
  }
  return within(row);
};

describe('user administration flow', () => {
  it('does not fetch users for a viewer route', async () => {
    const listUsers = vi.fn().mockResolvedValue(finalPage);
    renderUsers(createTestUsersApi({ listUsers }), 'viewer');

    expect(await screen.findByRole('heading', { level: 1, name: 'Overview' })).not.toBeNull();
    expect(listUsers).not.toHaveBeenCalled();
  });

  it('renders user administration in Persian RTL', async () => {
    globalThis.localStorage.setItem('stackplore.locale', 'fa');
    renderUsers(
      createTestUsersApi({
        listUsers: vi.fn().mockResolvedValue({ items: [administrator] }),
      }),
    );

    expect(await screen.findByRole('heading', { level: 1, name: 'کاربران' })).not.toBeNull();
    expect(await screen.findByRole('button', { name: 'افزودن کاربر' })).not.toBeNull();
    expect(document.documentElement.dir).toBe('rtl');
  });

  it('marks and protects the last enabled administrator', async () => {
    renderUsers(
      createTestUsersApi({
        listUsers: vi.fn().mockResolvedValue({ items: [administrator] }),
      }),
    );

    expect(await screen.findByText('Last enabled administrator')).not.toBeNull();
    expect(
      screen
        .getByRole('button', {
          name: 'Cannot delete Operations, the last enabled administrator',
        })
        .hasAttribute('disabled'),
    ).toBe(true);
  });

  it('creates a user through one reviewed dialog and refreshes the list', async () => {
    const operator = userEvent.setup();
    const listUsers = vi
      .fn()
      .mockResolvedValueOnce({ items: [administrator] })
      .mockResolvedValueOnce(finalPage);
    const createUser = vi.fn().mockResolvedValue(viewer);
    renderUsers(createTestUsersApi({ createUser, listUsers }));

    await screen.findByText('Operations');
    await operator.click(screen.getByRole('button', { name: 'Add user' }));
    expect(screen.getByText('Exact impact')).not.toBeNull();
    await operator.type(screen.getByLabelText(/Display name/u), 'Observer');
    await operator.type(screen.getByLabelText(/Login name/u), 'observer');
    await operator.type(screen.getByLabelText(/New password/u), 'viewer-password');
    await operator.type(screen.getByLabelText(/Confirm password/u), 'viewer-password');
    await operator.type(screen.getByLabelText(/Your current password/u), 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Create user' }));

    expect(createUser).toHaveBeenCalledWith({
      currentPassword: 'admin-password',
      displayName: 'Observer',
      loginName: 'observer',
      password: 'viewer-password',
      role: 'viewer',
    });
    expect(await screen.findByText('Observer')).not.toBeNull();
    expect(listUsers).toHaveBeenCalledTimes(2);
  });

  it('runs edit, password reset, session revoke, and delete with exact inputs', async () => {
    const operator = userEvent.setup();
    const listUsers = vi.fn().mockResolvedValue(finalPage);
    const updateUser = vi.fn().mockResolvedValue({ ...viewer, displayName: 'Night Observer' });
    const resetUserPassword = vi.fn().mockResolvedValue(undefined);
    const revokeUserSessions = vi.fn().mockResolvedValue(undefined);
    const deleteUser = vi.fn().mockResolvedValue(undefined);
    renderUsers(
      createTestUsersApi({
        deleteUser,
        listUsers,
        resetUserPassword,
        revokeUserSessions,
        updateUser,
      }),
    );

    let row = await userRow('Observer');
    await operator.click(row.getByRole('button', { name: 'Edit Observer' }));
    expect(screen.getByText(/display-name-only change keeps sessions/u)).not.toBeNull();
    const displayName = screen.getByLabelText(/Display name/u);
    await operator.clear(displayName);
    await operator.type(displayName, 'Night Observer');
    await operator.type(screen.getByLabelText(/Your current password/u), 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    expect(updateUser).toHaveBeenCalledWith(viewer, {
      currentPassword: 'admin-password',
      displayName: 'Night Observer',
    });

    row = await userRow('Observer');
    await operator.click(row.getByRole('button', { name: 'Reset password for Observer' }));
    expect(screen.getByText(/revokes every session and outstanding proof/u)).not.toBeNull();
    await operator.type(screen.getByLabelText(/New password/u), 'replacement-password');
    await operator.type(screen.getByLabelText(/Confirm password/u), 'replacement-password');
    await operator.type(screen.getByLabelText(/Your current password/u), 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Reset password' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    expect(resetUserPassword).toHaveBeenCalledWith(viewer, {
      currentPassword: 'admin-password',
      newPassword: 'replacement-password',
    });

    row = await userRow('Observer');
    await operator.click(row.getByRole('button', { name: 'Revoke sessions for Observer' }));
    expect(
      screen.getByText(/account, role, access status, and password stay unchanged/u),
    ).not.toBeNull();
    await operator.type(screen.getByLabelText(/Your current password/u), 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Revoke all sessions' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    expect(revokeUserSessions).toHaveBeenCalledWith(viewer, 'admin-password');

    row = await userRow('Observer');
    await operator.click(row.getByRole('button', { name: 'Delete Observer' }));
    expect(screen.getByText(/cannot be undone/u)).not.toBeNull();
    await operator.type(screen.getByLabelText(/Your current password/u), 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Delete user' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    expect(deleteUser).toHaveBeenCalledWith(viewer, 'admin-password');
  });

  it('shows a stable stale-or-last-admin conflict and clears the current password', async () => {
    const operator = userEvent.setup();
    const updateUser = vi.fn().mockRejectedValue(conflict());
    renderUsers(
      createTestUsersApi({
        listUsers: vi.fn().mockResolvedValue(finalPage),
        updateUser,
      }),
    );

    const row = await userRow('Observer');
    await operator.click(row.getByRole('button', { name: 'Edit Observer' }));
    const displayName = screen.getByLabelText(/Display name/u);
    await operator.clear(displayName);
    await operator.type(displayName, 'Changed Observer');
    const currentPassword = screen.getByLabelText(/Your current password/u);
    await operator.type(currentPassword, 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Save changes' }));

    expect(
      await screen.findByText(/record changed, or the last-administrator safety rule/u),
    ).not.toBeNull();
    expect((currentPassword as HTMLInputElement).value).toBe('');
    expect(screen.getByRole('dialog')).not.toBeNull();
  });

  it('clears every password field after a failed create attempt', async () => {
    const operator = userEvent.setup();
    const createUser = vi.fn().mockRejectedValue(new Error('offline'));
    renderUsers(
      createTestUsersApi({
        createUser,
        listUsers: vi.fn().mockResolvedValue({ items: [administrator] }),
      }),
    );

    await screen.findByText('Operations');
    await operator.click(screen.getByRole('button', { name: 'Add user' }));
    await operator.type(screen.getByLabelText(/Display name/u), 'Observer');
    await operator.type(screen.getByLabelText(/Login name/u), 'observer');
    const password = screen.getByLabelText(/New password/u);
    const confirmation = screen.getByLabelText(/Confirm password/u);
    const currentPassword = screen.getByLabelText(/Your current password/u);
    await operator.type(password, 'viewer-password');
    await operator.type(confirmation, 'viewer-password');
    await operator.type(currentPassword, 'admin-password');
    await operator.click(screen.getByRole('button', { name: 'Create user' }));

    expect(await screen.findByText(/action could not be completed/u)).not.toBeNull();
    expect((password as HTMLInputElement).value).toBe('');
    expect((confirmation as HTMLInputElement).value).toBe('');
    expect((currentPassword as HTMLInputElement).value).toBe('');
  });
});
