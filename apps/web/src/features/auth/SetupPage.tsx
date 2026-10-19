import { sliceCodePoints } from '@stackplore/api-client';
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  Field,
  Input,
} from '@stackplore/ui';
import { useRouter } from '@tanstack/react-router';
import { useEffect, useRef, useState, type SyntheticEvent } from 'react';

import { AuthBootPage } from './AuthBootPage';
import { classifyAuthFailure } from './auth-api';
import { useAuth } from './auth-context';
import { clearPasswordFields, isValidPasswordText, readFormText } from './auth-form';
import { useLocale, type TextMessageKey } from '../../i18n';

const failureMessage = (kind: ReturnType<typeof classifyAuthFailure>['kind']): TextMessageKey => {
  switch (kind) {
    case 'forbidden':
      return 'auth.setup.error.bootstrap';
    case 'rate_limited':
      return 'auth.error.rateLimited';
    case 'setup_closed':
      return 'auth.setup.error.closed';
    case 'unavailable':
      return 'auth.error.unavailable';
    case 'validation':
      return 'auth.error.validation';
    default:
      return 'auth.error.unexpected';
  }
};

export function SetupPage() {
  const { retry, setup, snapshot } = useAuth();
  const { t } = useLocale();
  const router = useRouter();
  const errorRef = useRef<HTMLDivElement>(null);
  const [errorKey, setErrorKey] = useState<TextMessageKey | null>(null);
  const [fieldErrors, setFieldErrors] = useState<ReadonlySet<string>>(new Set());
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (errorKey !== null) {
      errorRef.current?.focus();
    }
  }, [errorKey]);

  if (snapshot.status === 'loading' || snapshot.status === 'unavailable') {
    return <AuthBootPage />;
  }
  if (snapshot.status !== 'setup_required') {
    return null;
  }

  const submit = async (form: HTMLFormElement) => {
    const data = new FormData(form);
    const displayName = readFormText(data, 'display_name').trim();
    const loginName = readFormText(data, 'login_name').trim();
    const password = readFormText(data, 'password');
    const passwordConfirmation = readFormText(data, 'password_confirmation');
    const bootstrapProof = readFormText(data, 'bootstrap_proof');
    setErrorKey(null);
    setFieldErrors(new Set());
    if (password !== passwordConfirmation) {
      setFieldErrors(new Set(['password', 'password_confirmation']));
      setErrorKey('auth.setup.error.passwordMismatch');
      clearPasswordFields(form);
      return;
    }
    if (!isValidPasswordText(password)) {
      setFieldErrors(new Set(['password']));
      setErrorKey('auth.error.validation');
      clearPasswordFields(form);
      return;
    }

    setSubmitting(true);
    try {
      await setup({
        displayName,
        loginName,
        password,
        ...(snapshot.bootstrapProofRequired ? { bootstrapProof } : {}),
      });
      await router.navigate({ replace: true, to: '/overview' });
      await router.invalidate();
    } catch (error) {
      const failure = classifyAuthFailure(error, 'setup');
      setFieldErrors(new Set(failure.fields));
      setErrorKey(failureMessage(failure.kind));
      if (failure.kind === 'setup_closed') {
        await retry();
        await router.invalidate();
      }
    } finally {
      clearPasswordFields(form);
      setSubmitting(false);
    }
  };

  const handleSubmit = (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!submitting) {
      void submit(event.currentTarget);
    }
  };

  return (
    <div className="auth-page">
      <Card className="auth-card" tone="raised">
        <CardHeader>
          <p className="route-page__eyebrow">{t('auth.setup.eyebrow')}</p>
          <h1 className="ui-card__title">{t('route.setup.title')}</h1>
          <CardDescription>{t('auth.setup.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="auth-form" aria-busy={submitting || undefined} onSubmit={handleSubmit}>
            {errorKey === null ? null : (
              <div ref={errorRef} className="auth-form__error" role="alert" tabIndex={-1}>
                {t(errorKey)}
              </div>
            )}
            <Field
              label={t('auth.field.displayName')}
              error={fieldErrors.has('display_name') ? t('auth.field.invalid') : undefined}
              required
            >
              <Input
                autoComplete="name"
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
              error={fieldErrors.has('login_name') ? t('auth.field.invalid') : undefined}
              required
            >
              <Input
                autoCapitalize="none"
                autoComplete="username"
                dir="ltr"
                maxLength={64}
                minLength={3}
                name="login_name"
                pattern="[A-Za-z0-9](?:[A-Za-z0-9._-]{1,62}[A-Za-z0-9])"
                spellCheck={false}
              />
            </Field>
            <Field
              label={t('auth.field.password')}
              description={t('auth.field.passwordHelp')}
              error={fieldErrors.has('password') ? t('auth.field.invalid') : undefined}
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
            <Field
              label={t('auth.field.passwordConfirmation')}
              error={fieldErrors.has('password_confirmation') ? t('auth.field.invalid') : undefined}
              required
            >
              <Input
                autoComplete="new-password"
                dir="ltr"
                maxLength={128}
                name="password_confirmation"
                type="password"
              />
            </Field>
            {snapshot.bootstrapProofRequired ? (
              <Field
                label={t('auth.field.bootstrapProof')}
                description={t('auth.setup.bootstrapHelp')}
                error={fieldErrors.has('bootstrap_proof') ? t('auth.field.invalid') : undefined}
                required
              >
                <Input
                  autoComplete="off"
                  dir="ltr"
                  maxLength={43}
                  minLength={43}
                  name="bootstrap_proof"
                  pattern="[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]"
                  type="password"
                />
              </Field>
            ) : null}
            <Button className="auth-form__submit" loading={submitting} type="submit">
              {t('auth.setup.submit')}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
