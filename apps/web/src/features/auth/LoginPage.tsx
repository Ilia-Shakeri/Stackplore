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

interface LoginPageProps {
  readonly next?: string;
}

const failureMessage = (kind: ReturnType<typeof classifyAuthFailure>['kind']): TextMessageKey => {
  switch (kind) {
    case 'credentials':
      return 'auth.login.error.credentials';
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

export function LoginPage({ next }: LoginPageProps) {
  const { login, snapshot } = useAuth();
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
  if (snapshot.status !== 'unauthenticated') {
    return null;
  }

  const submit = async (form: HTMLFormElement) => {
    const data = new FormData(form);
    const loginName = readFormText(data, 'login_name').trim();
    const password = readFormText(data, 'password');
    setErrorKey(null);
    setFieldErrors(new Set());
    if (!isValidPasswordText(password)) {
      setFieldErrors(new Set(['password']));
      setErrorKey('auth.error.validation');
      clearPasswordFields(form);
      return;
    }
    setSubmitting(true);
    try {
      await login({ loginName, password });
      if (next === undefined) {
        await router.navigate({ replace: true, to: '/overview' });
      } else {
        router.history.replace(next);
      }
      await router.invalidate();
    } catch (error) {
      const failure = classifyAuthFailure(error, 'login');
      setFieldErrors(new Set(failure.fields));
      setErrorKey(failureMessage(failure.kind));
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
          <p className="route-page__eyebrow">{t('auth.login.eyebrow')}</p>
          <h1 className="ui-card__title">{t('route.login.title')}</h1>
          <CardDescription>{t('auth.login.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="auth-form" aria-busy={submitting || undefined} onSubmit={handleSubmit}>
            {errorKey === null ? null : (
              <div ref={errorRef} className="auth-form__error" role="alert" tabIndex={-1}>
                {t(errorKey)}
              </div>
            )}
            <Field
              label={t('auth.field.loginName')}
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
              error={fieldErrors.has('password') ? t('auth.field.invalid') : undefined}
              required
            >
              <Input
                autoComplete="current-password"
                dir="ltr"
                maxLength={128}
                name="password"
                type="password"
              />
            </Field>
            <Button className="auth-form__submit" loading={submitting} type="submit">
              {t('auth.login.submit')}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
