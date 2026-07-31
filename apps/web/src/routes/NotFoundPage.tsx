import { Link } from '@tanstack/react-router';

import { useLocale } from '../i18n';

export function NotFoundPage() {
  const { t } = useLocale();

  return (
    <div className="route-page route-page--centered">
      <section className="not-found" aria-labelledby="not-found-title">
        <p className="route-page__eyebrow">404</p>
        <h1 id="not-found-title">{t('route.notFound.title')}</h1>
        <p>{t('route.notFound.description')}</p>
        <Link className="text-link" to="/overview">
          {t('route.notFound.return')}
        </Link>
      </section>
    </div>
  );
}
