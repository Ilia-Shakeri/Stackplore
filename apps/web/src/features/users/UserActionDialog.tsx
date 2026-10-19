import { sliceCodePoints } from '@stackplore/api-client';
import {
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  Field,
  Input,
  Select,
  SelectOption,
} from '@stackplore/ui';
import { useEffect, useRef, useState, type SyntheticEvent } from 'react';

import { classifyUsersFailure, type UsersFailureKind } from './users-api';
import { useUsersApi } from './users-context';
import type { CurrentUser, UserRole } from '../auth/auth-api';
import { isValidPasswordText, readFormText } from '../auth/auth-form';
import { useLocale, type TextMessageKey } from '../../i18n';

export type UserAction =
  | { readonly kind: 'create' }
  | { readonly kind: 'delete'; readonly user: CurrentUser }
  | { readonly kind: 'edit'; readonly user: CurrentUser }
  | { readonly kind: 'reset_password'; readonly user: CurrentUser }
  | { readonly kind: 'revoke_sessions'; readonly user: CurrentUser };

type DialogError = UsersFailureKind | 'no_changes' | 'password_mismatch';

export interface UserActionDialogProps {
  readonly action: UserAction;
  readonly lastEnabledAdministrator: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onSessionLost: () => Promise<void>;
  readonly onSuccess: (action: UserAction) => Promise<void>;
  readonly open: boolean;
}

const errorMessage = (error: DialogError): TextMessageKey => {
  switch (error) {
    case 'conflict':
      return 'users.error.conflict';
    case 'forbidden':
      return 'users.error.forbidden';
    case 'not_found':
      return 'users.error.notFound';
    case 'rate_limited':
      return 'users.error.rateLimited';
    case 'session':
      return 'users.error.session';
    case 'unavailable':
      return 'users.error.unavailable';
    case 'validation':
      return 'users.error.validation';
    case 'no_changes':
      return 'users.edit.error.noChanges';
    case 'password_mismatch':
      return 'users.error.passwordMismatch';
    default:
      return 'users.error.unexpected';
  }
};

const titleMessage = (action: UserAction): TextMessageKey => {
  switch (action.kind) {
    case 'create':
      return 'users.create.title';
    case 'edit':
      return 'users.edit.title';
    case 'delete':
      return 'users.delete.title';
    case 'reset_password':
      return 'users.reset.title';
    case 'revoke_sessions':
      return 'users.revoke.title';
  }
};

const descriptionMessage = (action: UserAction): TextMessageKey => {
  switch (action.kind) {
    case 'create':
      return 'users.create.description';
    case 'edit':
      return 'users.edit.description';
    case 'delete':
      return 'users.delete.description';
    case 'reset_password':
      return 'users.reset.description';
    case 'revoke_sessions':
      return 'users.revoke.description';
  }
};

const impactMessage = (action: UserAction): TextMessageKey => {
  switch (action.kind) {
    case 'create':
      return 'users.create.impact';
    case 'edit':
      return 'users.edit.impact';
    case 'delete':
      return 'users.delete.impact';
    case 'reset_password':
      return 'users.reset.impact';
    case 'revoke_sessions':
      return 'users.revoke.impact';
  }
};

const submitMessage = (action: UserAction): TextMessageKey => {
  switch (action.kind) {
    case 'create':
      return 'users.create.submit';
    case 'edit':
      return 'users.edit.submit';
    case 'delete':
      return 'users.delete.submit';
    case 'reset_password':
      return 'users.reset.submit';
    case 'revoke_sessions':
      return 'users.revoke.submit';
  }
};

const clearSecrets = (form: HTMLFormElement) => {
  for (const input of form.querySelectorAll<HTMLInputElement>('input[type="password"]')) {
    input.value = '';
  }
};

const targetValues = (action: UserAction) =>
  action.kind === 'create' ? {} : { name: action.user.displayName };

export function UserActionDialog({
  action,
  lastEnabledAdministrator,
  onOpenChange,
  onSessionLost,
  onSuccess,
  open,
}: UserActionDialogProps) {
  const api = useUsersApi();
  const { t } = useLocale();
  const errorRef = useRef<HTMLDivElement>(null);
  const submitLockRef = useRef(false);
  const [error, setError] = useState<DialogError | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const formId = `user-action-${action.kind}`;
  const target = targetValues(action);

  useEffect(() => {
    if (error !== null) {
      errorRef.current?.focus();
    }
  }, [error]);

  const submit = async (form: HTMLFormElement) => {
    submitLockRef.current = true;
    const data = new FormData(form);
    const currentPassword = readFormText(data, 'current_password');
    setError(null);
    setSubmitting(true);
    try {
      if (!isValidPasswordText(currentPassword)) {
        setError('validation');
        return;
      }
      switch (action.kind) {
        case 'create': {
          const password = readFormText(data, 'password');
          if (password !== readFormText(data, 'password_confirmation')) {
            setError('password_mismatch');
            return;
          }
          if (!isValidPasswordText(password)) {
            setError('validation');
            return;
          }
          await api.createUser({
            currentPassword,
            displayName: readFormText(data, 'display_name').trim(),
            loginName: readFormText(data, 'login_name').trim(),
            password,
            role: readFormText(data, 'role') as UserRole,
          });
          break;
        }
        case 'edit': {
          const displayName = readFormText(data, 'display_name').trim();
          const enabled = readFormText(data, 'enabled') === 'true';
          const loginName = readFormText(data, 'login_name').trim();
          const role = readFormText(data, 'role') as UserRole;
          const changes = {
            currentPassword,
            ...(displayName === action.user.displayName ? {} : { displayName }),
            ...(enabled === action.user.enabled ? {} : { enabled }),
            ...(loginName === action.user.loginName ? {} : { loginName }),
            ...(role === action.user.role ? {} : { role }),
          };
          if (Object.keys(changes).length === 1) {
            setError('no_changes');
            return;
          }
          await api.updateUser(action.user, changes);
          break;
        }
        case 'delete':
          await api.deleteUser(action.user, currentPassword);
          break;
        case 'reset_password': {
          const newPassword = readFormText(data, 'new_password');
          if (newPassword !== readFormText(data, 'password_confirmation')) {
            setError('password_mismatch');
            return;
          }
          if (!isValidPasswordText(newPassword)) {
            setError('validation');
            return;
          }
          await api.resetUserPassword(action.user, { currentPassword, newPassword });
          break;
        }
        case 'revoke_sessions':
          await api.revokeUserSessions(action.user, currentPassword);
          break;
      }
      form.reset();
      await onSuccess(action);
    } catch (reason) {
      const kind = classifyUsersFailure(reason);
      setError(kind);
      if (kind === 'session') {
        await onSessionLost();
      }
    } finally {
      clearSecrets(form);
      submitLockRef.current = false;
      setSubmitting(false);
    }
  };

  const handleSubmit = (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!submitLockRef.current) {
      void submit(event.currentTarget);
    }
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (submitLockRef.current) {
      return;
    }
    setError(null);
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        closeLabel={t('users.dialog.close')}
        description={t(descriptionMessage(action), target)}
        footer={
          <>
            <DialogClose asChild>
              <Button disabled={submitting} variant="outline">
                {t('users.dialog.cancel')}
              </Button>
            </DialogClose>
            <Button
              form={formId}
              loading={submitting}
              type="submit"
              variant={action.kind === 'delete' ? 'danger' : 'primary'}
            >
              {t(submitMessage(action))}
            </Button>
          </>
        }
        size={action.kind === 'create' || action.kind === 'edit' ? 'lg' : 'md'}
        title={t(titleMessage(action), target)}
      >
        <form
          id={formId}
          className="users-dialog__form"
          aria-busy={submitting || undefined}
          onSubmit={handleSubmit}
        >
          <div className="users-dialog__impact">
            <strong>{t('users.dialog.impactTitle')}</strong>
            <p>{t(impactMessage(action), target)}</p>
          </div>
          {lastEnabledAdministrator && (action.kind === 'edit' || action.kind === 'delete') ? (
            <div className="users-dialog__safety" role="note">
              {t('users.lastAdministrator.safety')}
            </div>
          ) : null}
          {error === null ? null : (
            <div ref={errorRef} className="auth-form__error" role="alert" tabIndex={-1}>
              {t(errorMessage(error))}
            </div>
          )}

          {action.kind === 'create' ? (
            <>
              <Field label={t('auth.field.displayName')} required>
                <Input
                  autoComplete="off"
                  maxLength={512}
                  name="display_name"
                  onInput={(event) => {
                    event.currentTarget.value = sliceCodePoints(event.currentTarget.value, 256);
                  }}
                />
              </Field>
              <Field
                label={t('auth.field.loginName')}
                description={t('auth.setup.loginHelp')}
                required
              >
                <Input
                  autoCapitalize="none"
                  autoComplete="off"
                  dir="ltr"
                  maxLength={64}
                  minLength={3}
                  name="login_name"
                  pattern="[A-Za-z0-9](?:[A-Za-z0-9._-]{1,62}[A-Za-z0-9])"
                  spellCheck={false}
                />
              </Field>
              <Field label={t('users.field.role')} required>
                <Select defaultValue="viewer" name="role">
                  <SelectOption value="viewer">{t('auth.role.viewer')}</SelectOption>
                  <SelectOption value="administrator">{t('auth.role.administrator')}</SelectOption>
                </Select>
              </Field>
              <Field
                label={t('users.field.newPassword')}
                description={t('auth.field.passwordHelp')}
                required
              >
                <Input
                  autoComplete="new-password"
                  dir="ltr"
                  maxLength={128}
                  name="password"
                  type="password"
                />
              </Field>
              <Field label={t('auth.field.passwordConfirmation')} required>
                <Input
                  autoComplete="new-password"
                  dir="ltr"
                  maxLength={128}
                  name="password_confirmation"
                  type="password"
                />
              </Field>
            </>
          ) : null}

          {action.kind === 'edit' ? (
            <>
              <Field label={t('auth.field.displayName')} required>
                <Input
                  autoComplete="off"
                  defaultValue={action.user.displayName}
                  maxLength={512}
                  onInput={(event) => {
                    event.currentTarget.value = sliceCodePoints(event.currentTarget.value, 256);
                  }}
                  name="display_name"
                />
              </Field>
              <Field
                label={t('auth.field.loginName')}
                description={t('auth.setup.loginHelp')}
                required
              >
                <Input
                  autoCapitalize="none"
                  autoComplete="off"
                  defaultValue={action.user.loginName}
                  dir="ltr"
                  maxLength={64}
                  minLength={3}
                  name="login_name"
                  pattern="[A-Za-z0-9](?:[A-Za-z0-9._-]{1,62}[A-Za-z0-9])"
                  spellCheck={false}
                />
              </Field>
              <Field label={t('users.field.role')} required>
                <Select defaultValue={action.user.role} name="role">
                  <SelectOption
                    disabled={lastEnabledAdministrator && action.user.role === 'administrator'}
                    value="viewer"
                  >
                    {t('auth.role.viewer')}
                  </SelectOption>
                  <SelectOption value="administrator">{t('auth.role.administrator')}</SelectOption>
                </Select>
              </Field>
              <Field label={t('users.field.status')} required>
                <Select defaultValue={String(action.user.enabled)} name="enabled">
                  <SelectOption value="true">{t('users.status.enabled')}</SelectOption>
                  <SelectOption
                    disabled={lastEnabledAdministrator && action.user.enabled}
                    value="false"
                  >
                    {t('users.status.disabled')}
                  </SelectOption>
                </Select>
              </Field>
            </>
          ) : null}

          {action.kind === 'reset_password' ? (
            <>
              <Field
                label={t('users.field.newPassword')}
                description={t('auth.field.passwordHelp')}
                required
              >
                <Input
                  autoComplete="new-password"
                  dir="ltr"
                  maxLength={128}
                  name="new_password"
                  type="password"
                />
              </Field>
              <Field label={t('auth.field.passwordConfirmation')} required>
                <Input
                  autoComplete="new-password"
                  dir="ltr"
                  maxLength={128}
                  name="password_confirmation"
                  type="password"
                />
              </Field>
            </>
          ) : null}

          <Field
            label={t('users.field.currentPassword')}
            description={t('users.field.currentPasswordHelp')}
            required
          >
            <Input
              autoComplete="current-password"
              dir="ltr"
              maxLength={128}
              name="current_password"
              type="password"
            />
          </Field>
        </form>
      </DialogContent>
    </Dialog>
  );
}
