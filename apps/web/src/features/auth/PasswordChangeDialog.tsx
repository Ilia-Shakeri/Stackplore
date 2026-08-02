import { Button, Dialog, DialogClose, DialogContent, Field, Input, useToast } from '@stackplore/ui';
import { useRouter } from '@tanstack/react-router';
import { useEffect, useRef, useState, type SyntheticEvent } from 'react';

import { classifyAuthFailure, isUnauthorizedFailure } from './auth-api';
import { useAuth } from './auth-context';
import { clearPasswordFields, isValidPasswordText, readFormText } from './auth-form';
import { useLocale, type TextMessageKey } from '../../i18n';

export interface PasswordChangeDialogProps {
  readonly onOpenChange: (open: boolean) => void;
  readonly open: boolean;
}

type PasswordError = 'mismatch' | 'password' | 'validation' | TextMessageKey;

const failureMessage = (kind: ReturnType<typeof classifyAuthFailure>['kind']): TextMessageKey => {
  switch (kind) {
    case 'forbidden':
      return 'auth.password.error.current';
    case 'rate_limited':
      return 'auth.error.rateLimited';
    case 'unavailable':
      return 'auth.error.unavailable';
    case 'validation':
      return 'auth.error.validation';
    default:
      return 'auth.error.unexpected';
  }
};

const errorMessage = (error: PasswordError): TextMessageKey => {
  switch (error) {
    case 'mismatch':
      return 'auth.password.error.mismatch';
    case 'password':
      return 'auth.password.error.current';
    case 'validation':
      return 'auth.error.validation';
    default:
      return error;
  }
};

export function PasswordChangeDialog({ onOpenChange, open }: PasswordChangeDialogProps) {
  const { changePassword, retry } = useAuth();
  const { notify } = useToast();
  const { t } = useLocale();
  const router = useRouter();
  const errorRef = useRef<HTMLDivElement>(null);
  const submitLockRef = useRef(false);
  const [error, setError] = useState<PasswordError | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const formId = 'own-password-change';

  useEffect(() => {
    if (error !== null) {
      errorRef.current?.focus();
    }
  }, [error]);

  const submit = async (form: HTMLFormElement) => {
    submitLockRef.current = true;
    const data = new FormData(form);
    const currentPassword = readFormText(data, 'current_password');
    const newPassword = readFormText(data, 'new_password');
    const confirmation = readFormText(data, 'password_confirmation');
    setError(null);
    setSubmitting(true);
    try {
      if (newPassword !== confirmation) {
        setError('mismatch');
        return;
      }
      if (!isValidPasswordText(currentPassword) || !isValidPasswordText(newPassword)) {
        setError('validation');
        return;
      }
      await changePassword(currentPassword, newPassword);
      form.reset();
      onOpenChange(false);
      notify({ title: t('auth.password.success'), tone: 'success' });
      await router.invalidate();
    } catch (reason) {
      if (isUnauthorizedFailure(reason)) {
        onOpenChange(false);
        await retry();
        await router.invalidate();
        return;
      }
      setError(failureMessage(classifyAuthFailure(reason, 'session').kind));
    } finally {
      clearPasswordFields(form);
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
        closeLabel={t('auth.password.close')}
        description={t('auth.password.description')}
        footer={
          <>
            <DialogClose asChild>
              <Button disabled={submitting} variant="outline">
                {t('auth.password.cancel')}
              </Button>
            </DialogClose>
            <Button form={formId} loading={submitting} type="submit">
              {t('auth.password.submit')}
            </Button>
          </>
        }
        size="md"
        title={t('auth.password.title')}
      >
        <form
          id={formId}
          className="users-dialog__form"
          aria-busy={submitting || undefined}
          onSubmit={handleSubmit}
        >
          <div className="users-dialog__impact">
            <strong>{t('auth.password.impactTitle')}</strong>
            <p>{t('auth.password.impact')}</p>
          </div>
          {error === null ? null : (
            <div ref={errorRef} className="auth-form__error" role="alert" tabIndex={-1}>
              {t(errorMessage(error))}
            </div>
          )}
          <Field label={t('auth.password.current')} required>
            <Input
              autoComplete="current-password"
              dir="ltr"
              maxLength={128}
              name="current_password"
              type="password"
            />
          </Field>
          <Field label={t('auth.password.new')} description={t('auth.field.passwordHelp')} required>
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
        </form>
      </DialogContent>
    </Dialog>
  );
}
