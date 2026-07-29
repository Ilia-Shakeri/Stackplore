import type { ReactNode } from 'react';

interface PlannedPageProps {
  children?: ReactNode;
  description: string;
  eyebrow?: string;
  title: string;
  toolbar?: ReactNode;
}

export function PlannedPage({
  children,
  description,
  eyebrow = 'Planned workspace',
  title,
  toolbar,
}: PlannedPageProps) {
  return (
    <div className="route-page min-w-0">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p>{description}</p>
      </header>

      {toolbar}

      <section className="planned-state" aria-labelledby="planned-state-title">
        <div className="planned-state__mark" aria-hidden="true" />
        <div>
          <h2 id="planned-state-title">No live data yet</h2>
          <p>
            This route is part of the application shell. Its provider-backed workflow arrives in a
            later delivery phase.
          </p>
          {children}
        </div>
      </section>
    </div>
  );
}
