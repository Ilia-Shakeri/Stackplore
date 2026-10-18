import { ApiProblemError } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { act, render, screen, waitFor, within } from '@testing-library/react';
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
  it('counts administrators across the loaded cache rather than just the visible window', async () => {
    const secondAdministrator = {
      ...administrator,
      id: 'usr_admin_second',
      displayName: 'Second administrator',
      loginName: 'secondadmin',
    };
    const first = [
      administrator,
      ...Array.from({ length: 49 }, (_, index) => ({
        ...viewer,
        id: `usr_visible_${String(index)}`,
        displayName: `Visible ${String(index)}`,
        loginName: `visible${String(index)}`,
      })),
    ];
    const listUsers = vi
      .fn()
      .mockResolvedValueOnce({ items: first, nextCursor: 'users_next' })
      .mockResolvedValueOnce({ items: [secondAdministrator] });
    const user = userEvent.setup();
    const { container } = renderUsers(createTestUsersApi({ listUsers }));
    await waitFor(() => {
      expect(container.querySelectorAll('tbody tr')).toHaveLength(50);
    });
    await user.click(screen.getByRole('button', { name: 'Load more users' }));
    await screen.findByText('Second administrator');
    await user.click(screen.getByRole('button', { name: 'Previous users' }));
    const row = within(screen.getByRole('table')).getByText('Operations').closest('tr');
    if (row === null) throw new Error('Administrator row missing.');
    expect(
      within(row).getByRole<HTMLButtonElement>('button', { name: 'Delete Operations' }).disabled,
    ).toBe(false);
    expect(screen.queryByText('Last enabled administrator')).toBeNull();
  });

  it('returns to the first visible page after a successful account action', async () => {
    const first = [
      administrator,
      ...Array.from({ length: 49 }, (_, index) => ({
        ...viewer,
        id: `usr_visible_${String(index)}`,
        displayName: `Visible ${String(index)}`,
        loginName: `visible${String(index)}`,
      })),
    ];
    const last = {
      ...viewer,
      id: 'usr_last_viewer',
      displayName: 'Last observer',
      loginName: 'lastobserver',
    };
    const listUsers = vi
      .fn()
      .mockResolvedValueOnce({ items: first, nextCursor: 'users_next' })
      .mockResolvedValueOnce({ items: [last] })
      .mockResolvedValueOnce({ items: first, nextCursor: 'users_next' });
    const revokeUserSessions = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    const { container } = renderUsers(createTestUsersApi({ listUsers, revokeUserSessions }));
    await waitFor(() => {
      expect(container.querySelectorAll('tbody tr')).toHaveLength(50);
    });
    await user.click(screen.getByRole('button', { name: 'Load more users' }));
    const row = await userRow('Last observer');
    await user.click(row.getByRole('button', { name: 'Revoke sessions for Last observer' }));
    await user.type(screen.getByLabelText(/Your current password/u), 'admin-password');
    await user.click(screen.getByRole('button', { name: 'Revoke all sessions' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    await waitFor(() => {
      expect(container.querySelectorAll('tbody tr')).toHaveLength(50);
    });
    expect(within(screen.getByRole('table')).getByText('Operations')).not.toBeNull();
    expect(screen.queryByText('Last observer')).toBeNull();
    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Previous users' }).disabled).toBe(
      true,
    );
    expect(revokeUserSessions).toHaveBeenCalledWith(last, 'admin-password');
    expect(listUsers).toHaveBeenCalledTimes(3);
  });

  it('keeps the current window after a failed next request and retries from the same cursor', async () => {
    const last = {
      ...viewer,
      id: 'usr_last_viewer',
      displayName: 'Last observer',
      loginName: 'lastobserver',
    };
    const listUsers = vi
      .fn()
      .mockResolvedValueOnce({ items: [administrator, viewer], nextCursor: 'users_next' })
      .mockRejectedValueOnce(new Error('Unavailable.'))
      .mockResolvedValueOnce({ items: [last] });
    const user = userEvent.setup();
    const { container } = renderUsers(createTestUsersApi({ listUsers }));
    await screen.findByText('Observer');
    await user.click(screen.getByRole('button', { name: 'Load more users' }));
    await screen.findByText('More users could not be loaded. The current rows remain unchanged.');
    expect(container.querySelectorAll('tbody tr')).toHaveLength(2);
    expect(screen.getByText('Observer')).not.toBeNull();
    await user.click(screen.getByRole('button', { name: 'Load more users' }));
    await screen.findByText('Last observer');
    expect(container.querySelectorAll('tbody tr')).toHaveLength(3);
    expect(
      screen.queryByText('More users could not be loaded. The current rows remain unchanged.'),
    ).toBeNull();
    expect(listUsers).toHaveBeenNthCalledWith(3, expect.objectContaining({ cursor: 'users_next' }));
  });

  it('aborts an old next-page read and ignores its late authorization failure after refresh', async () => {
    let failOld!: (error: Error) => void;
    let oldSignal: AbortSignal | undefined;
    const fresh = { ...viewer, id: 'usr_fresh', displayName: 'Fresh list', loginName: 'freshlist' };
    const listUsers = vi
      .fn()
      .mockResolvedValueOnce({ items: [administrator, viewer], nextCursor: 'users_next' })
      .mockImplementationOnce((options: { signal?: AbortSignal }) => {
        oldSignal = options.signal;
        return new Promise((_, reject) => {
          failOld = reject;
        });
      })
      .mockResolvedValueOnce({ items: [fresh] });
    const user = userEvent.setup();
    renderUsers(createTestUsersApi({ listUsers }));
    await screen.findByText('Observer');
    await user.click(screen.getByRole('button', { name: 'Load more users' }));
    await user.click(screen.getByRole('button', { name: 'Refresh users' }));
    await screen.findByText('Fresh list');
    expect(oldSignal?.aborted).toBe(true);
    await act(async () => {
      failOld(
        new ApiProblemError({
          code: 'unauthenticated',
          status: 401,
          title: 'Sign in required',
          type: 'urn:stackplore:problem:unauthenticated',
          request_id: 'request_old_users_page',
        }),
      );
      await Promise.resolve();
    });
    expect(screen.getByText('Fresh list')).not.toBeNull();
    expect(
      screen.queryByText('More users could not be loaded. The current rows remain unchanged.'),
    ).toBeNull();
  });
  it.each(['en', 'fa'] as const)(
    'renders 103 accounts through bounded pages with complete access in %s',
    async (locale) => {
      localStorage.setItem('stackplore.locale', locale);
      const users = [
        administrator,
        ...Array.from({ length: 102 }, (_, index) => ({
          ...viewer,
          id: `usr_viewer_${String(index + 1)}`,
          loginName: `observer${String(index + 1)}`,
          displayName: `Observer ${String(index + 1).padStart(3, '0')}`,
        })),
      ];
      const listUsers = vi.fn().mockImplementation((options: { cursor?: string } = {}) => {
        const start =
          options.cursor === 'users_page_3' ? 100 : options.cursor === 'users_page_2' ? 50 : 0;
        return Promise.resolve({
          items: users.slice(start, start + 50),
          ...(start < 100 ? { nextCursor: start === 0 ? 'users_page_2' : 'users_page_3' } : {}),
        });
      });
      const user = userEvent.setup();
      const { container } = renderUsers(createTestUsersApi({ listUsers }));
      const rowCount = () => container.querySelectorAll('.users-table tbody tr').length;
      const loadLabel = locale === 'en' ? 'Load more users' : 'بارگذاری کاربران بیشتر';
      const nextLabel = locale === 'en' ? 'Next users' : 'کاربران بعدی';
      const previousLabel = locale === 'en' ? 'Previous users' : 'کاربران قبلی';
      const safetyLabel = locale === 'en' ? 'Last enabled administrator' : 'آخرین مدیر فعال';
      await waitFor(() => {
        expect(rowCount()).toBe(50);
      });
      expect(screen.queryByText(safetyLabel)).toBeNull();
      await user.click(screen.getByRole('button', { name: loadLabel }));
      await screen.findByText('Observer 050');
      expect(rowCount()).toBe(50);
      expect(within(screen.getByRole('table')).queryByText('Operations')).toBeNull();
      await user.click(screen.getByRole('button', { name: loadLabel }));
      await screen.findByText('Observer 102');
      expect(rowCount()).toBe(3);
      expect(listUsers).toHaveBeenCalledTimes(3);
      expect(screen.getByRole<HTMLButtonElement>('button', { name: nextLabel }).disabled).toBe(
        true,
      );
      await user.click(screen.getByRole('button', { name: previousLabel }));
      expect(rowCount()).toBe(50);
      expect(screen.getByText('Observer 050')).not.toBeNull();
      await user.click(screen.getByRole('button', { name: previousLabel }));
      expect(rowCount()).toBe(50);
      expect(within(screen.getByRole('table')).getByText('Operations')).not.toBeNull();
      expect(screen.getByText(safetyLabel)).not.toBeNull();
      const adminRow = within(screen.getByRole('table')).getByText('Operations').closest('tr');
      if (adminRow === null) throw new Error('Administrator row missing.');
      expect(
        within(adminRow).getByRole<HTMLButtonElement>('button', {
          name:
            locale === 'en'
              ? 'Cannot delete Operations, the last enabled administrator'
              : 'حذف Operations ممکن نیست؛ آخرین مدیر فعال است',
        }).disabled,
      ).toBe(true);
      await user.click(screen.getByRole('button', { name: nextLabel }));
      await user.click(screen.getByRole('button', { name: nextLabel }));
      expect(screen.getByText('Observer 102')).not.toBeNull();
      expect(rowCount()).toBe(3);
      expect(listUsers).toHaveBeenCalledTimes(3);
      await user.click(
        screen.getByRole('button', {
          name: locale === 'en' ? 'Refresh users' : 'تازه‌سازی کاربران',
        }),
      );
      await waitFor(() => {
        expect(rowCount()).toBe(50);
      });
      expect(within(screen.getByRole('table')).getByText('Operations')).not.toBeNull();
      expect(screen.queryByText('Observer 102')).toBeNull();
      expect(screen.queryByText(safetyLabel)).toBeNull();
      expect(listUsers).toHaveBeenCalledTimes(4);
      expect(screen.getByRole<HTMLButtonElement>('button', { name: previousLabel }).disabled).toBe(
        true,
      );
    },
  );
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
