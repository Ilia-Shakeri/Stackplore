import { ChevronRight } from '@stackplore/ui/icons';
import { Link, useRouterState } from '@tanstack/react-router';

import { getBreadcrumbs } from './navigation';
import { useLocale } from '../../i18n';
import { useProductSource, useProductView } from '../../features/product/product-context';
import {
  environmentNavigationSearch,
  readEnvironment,
} from '../../features/product/environment-navigation';

export function Breadcrumbs() {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useLocale();
  const { selectedSource } = useProductSource();
  const view = useProductView();
  const environment = readEnvironment(view.search);
  const breadcrumbs = getBreadcrumbs(pathname, t);

  if (breadcrumbs.length === 0) {
    return null;
  }

  return (
    <nav className="breadcrumbs" aria-label={t('shell.breadcrumb.label')}>
      <ol>
        {breadcrumbs.map((item, index) => {
          const current = index === breadcrumbs.length - 1;
          return (
            <li key={`${item.to ?? 'current'}-${item.label}`}>
              {index > 0 ? <ChevronRight data-ui-mirror-inline aria-hidden="true" /> : null}
              {item.to === undefined ? (
                <span aria-current={current ? 'page' : undefined}>{item.label}</span>
              ) : (
                <Link
                  className="breadcrumbs__link ui-focus-ring"
                  to={item.to}
                  search={environmentNavigationSearch(item.to, selectedSource, environment)}
                >
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
