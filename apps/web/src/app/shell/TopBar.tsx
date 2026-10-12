import { Bell, Search } from '@stackplore/ui/icons';
import { Link, useNavigate, useRouterState } from '@tanstack/react-router';
import { useState, type SyntheticEvent } from 'react';

import { Breadcrumbs } from './Breadcrumbs';
import { LocaleControl } from './LocaleControl';
import { MobileNav } from './MobileNav';
import { ThemeControl } from './ThemeControl';
import { UserMenu } from '../../features/auth/UserMenu';
import { useProductSource } from '../../features/product/product-context';
import { getPageTitle } from './navigation';
import { useLocale } from '../../i18n';

export function TopBar() {
  const [mobileNavigationOpen, setMobileNavigationOpen] = useState(false);
  const [query, setQuery] = useState('');
  const navigate = useNavigate();
  const { selectedSource } = useProductSource();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useLocale();
  const pageTitle = getPageTitle(pathname, t);
  const submitSearch = (event: SyntheticEvent<HTMLFormElement, SubmitEvent>) => {
    event.preventDefault();
    const q = query.trim();
    if (q === '') return;
    void navigate({
      to: '/search',
      search: { q, ...(selectedSource === undefined ? {} : { source: selectedSource }) },
    });
  };

  return (
    <header className="app-topbar">
      <div className="app-topbar__mobile-leading">
        <MobileNav
          open={mobileNavigationOpen}
          pathname={pathname}
          onOpenChange={setMobileNavigationOpen}
        />
        <span className="app-topbar__title">{pageTitle}</span>
      </div>
      <Breadcrumbs />
      <form className="app-topbar__search" role="search" onSubmit={submitSearch}>
        <input
          aria-label={t('product.search.label')}
          maxLength={200}
          onChange={(event) => {
            setQuery(event.target.value);
          }}
          placeholder={t('product.search.placeholder')}
          type="search"
          value={query}
        />
        <button className="ui-focus-ring" type="submit" aria-label={t('product.search.submit')}>
          <Search aria-hidden="true" size={17} />
        </button>
      </form>
      <div className="app-topbar__actions">
        <Link
          className="app-topbar__notification ui-focus-ring"
          to="/notifications"
          aria-label={t('shell.notifications.open')}
        >
          <Bell aria-hidden="true" size={18} />
        </Link>
        <LocaleControl />
        <ThemeControl />
        <UserMenu />
      </div>
    </header>
  );
}
