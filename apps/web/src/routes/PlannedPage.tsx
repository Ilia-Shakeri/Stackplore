import type { ReactNode } from 'react';

import { useLocale } from '../i18n';

interface PlannedPageProps {
  children?: ReactNode;
  description: string;
  eyebrow?: string;
  title: string;
  toolbar?: ReactNode;
}

export function PlannedPage({ children, description, eyebrow, title, toolbar }: PlannedPageProps) {
  const { t } = useLocale();

  return (
    <div className="route-page min-w-0">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{eyebrow ?? t('route.planned.eyebrow')}</p>
        <h1>{title}</h1>
        <p>{description}</p>
      </header>

      {toolbar}

      <section className="planned-state" aria-labelledby="planned-state-title">
        <div className="planned-state__mark" aria-hidden="true" />
        <div>
          <h2 id="planned-state-title">{t('route.planned.title')}</h2>
          <p>{t('route.planned.description')}</p>
          {children}
        </div>
      </section>
    </div>
  );
}
