import {
  Button,
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownLabel,
  DropdownSeparator,
  DropdownTrigger,
  TechnicalText,
  useToast,
} from '@stackplore/ui';
import { useRouter } from '@tanstack/react-router';
import { useRef, useState } from 'react';

import { PasswordChangeDialog } from './PasswordChangeDialog';
import { useAuth } from './auth-context';
import { useLocale } from '../../i18n';

export function UserMenu() {
  const { logout, snapshot } = useAuth();
  const { notify } = useToast();
  const { t } = useLocale();
  const router = useRouter();
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);

  if (snapshot.status === 'guest_demo')
    return <span className="guest-demo-badge">{t('demo.mode.guest')}</span>;

  if (snapshot.status !== 'authenticated') {
    return null;
  }

  const handleLogout = async () => {
    if (submitting) {
      return;
    }
    setSubmitting(true);
    try {
      await logout();
      await router.navigate({ replace: true, to: '/login' });
      await router.invalidate();
    } catch {
      notify({
        description: t('auth.logout.errorBody'),
        title: t('auth.logout.errorTitle'),
        tone: 'danger',
      });
    } finally {
      setSubmitting(false);
    }
  };

  const role = t(
    snapshot.user.role === 'administrator' ? 'auth.role.administrator' : 'auth.role.viewer',
  );

  return (
    <>
      <Dropdown>
        <DropdownTrigger asChild>
          <Button
            ref={triggerRef}
            className="user-menu__trigger"
            variant="ghost"
            aria-label={t('auth.userMenu.open')}
          >
            <span className="user-menu__name">{snapshot.user.displayName}</span>
          </Button>
        </DropdownTrigger>
        <DropdownContent
          align="end"
          className="user-menu__content"
          onCloseAutoFocus={(event) => {
            if (passwordOpen) event.preventDefault();
          }}
        >
          <DropdownLabel>
            <span className="user-menu__identity">
              <strong>{snapshot.user.displayName}</strong>
              <TechnicalText>{snapshot.user.loginName}</TechnicalText>
              <span>{role}</span>
            </span>
          </DropdownLabel>
          <DropdownSeparator />
          <DropdownItem
            disabled={submitting}
            onSelect={() => {
              setPasswordOpen(true);
            }}
          >
            {t('auth.password.action')}
          </DropdownItem>
          <DropdownItem disabled={submitting} onSelect={() => void handleLogout()}>
            {submitting ? t('auth.logout.pending') : t('auth.logout.action')}
          </DropdownItem>
        </DropdownContent>
      </Dropdown>
      <PasswordChangeDialog
        open={passwordOpen}
        onReturnFocus={() => {
          const target = triggerRef.current ?? document.getElementById('main-content');
          target?.focus();
        }}
        onOpenChange={(open) => {
          setPasswordOpen(open);
        }}
      />
    </>
  );
}
