import { ChevronRight } from '@stackplore/ui/icons';
import { Link, useRouterState } from '@tanstack/react-router';

import { getBreadcrumbs } from './navigation';

export function Breadcrumbs() {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const breadcrumbs = getBreadcrumbs(pathname);

  if (breadcrumbs.length === 0) {
    return null;
  }

  return (
    <nav className="breadcrumbs" aria-label="Breadcrumb">
      <ol>
        {breadcrumbs.map((item, index) => {
          const current = index === breadcrumbs.length - 1;
          return (
            <li key={`${item.to ?? 'current'}-${item.label}`}>
              {index > 0 ? <ChevronRight aria-hidden="true" /> : null}
              {item.to === undefined ? (
                <span aria-current={current ? 'page' : undefined}>{item.label}</span>
              ) : (
                <Link className="breadcrumbs__link ui-focus-ring" to={item.to}>
                  {item.label}
                </Link>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
