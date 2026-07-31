import { Tooltip } from '@stackplore/ui';
import { Link } from '@tanstack/react-router';

import type { NavigationItem } from './navigation';
import { useLocale } from '../../i18n';

interface NavigationLinksProps {
  collapsed?: boolean;
  items: readonly NavigationItem[];
  onNavigate?: () => void;
}

export function NavigationLinks({ collapsed = false, items, onNavigate }: NavigationLinksProps) {
  const { direction, t } = useLocale();

  return (
    <ul className="shell-nav__list">
      {items.map((item) => {
        const Icon = item.icon;
        const label = t(item.labelKey);
        const link = (
          <Link
            className="shell-nav__link ui-focus-ring"
            to={item.to}
            activeOptions={{ exact: item.to !== '/settings' }}
            onClick={onNavigate}
          >
            <Icon className="shell-nav__icon" aria-hidden="true" />
            <span className={collapsed ? 'shell-nav__label ui-sr-only' : 'shell-nav__label'}>
              {label}
            </span>
          </Link>
        );

        return (
          <li key={item.to}>
            {collapsed ? (
              <Tooltip content={label} side={direction === 'rtl' ? 'left' : 'right'}>
                {link}
              </Tooltip>
            ) : (
              link
            )}
          </li>
        );
      })}
    </ul>
  );
}
