import { Dialog, DialogContent, DialogTrigger, Input } from '@stackplore/ui';
import { Bell, Command, Search } from '@stackplore/ui/icons';
import { useNavigate } from '@tanstack/react-router';
import { useEffect, useId, useRef, useState } from 'react';

import { getPrimaryNavigation, getSecondaryNavigation, type NavigationItem } from './navigation';
import { useAuth } from '../../features/auth/auth-context';
import { useProductSource } from '../../features/product/product-context';
import { useLocale } from '../../i18n';

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const previousFocus = useRef<HTMLElement | null>(null);
  const navigated = useRef(false);
  const activeOption = useRef<HTMLLIElement>(null);
  const listId = useId();
  const { snapshot } = useAuth();
  const { selectedSource } = useProductSource();
  const { locale, t } = useLocale();
  const navigate = useNavigate();
  const role = snapshot.status === 'authenticated' ? snapshot.user.role : 'viewer';
  const commands: readonly NavigationItem[] = [
    ...getPrimaryNavigation(role),
    { icon: Search, labelKey: 'route.search.title', to: '/search' },
    { icon: Bell, labelKey: 'route.notificationCenter.title', to: '/notifications' },
    ...getSecondaryNavigation(role),
  ];
  const needle = query.trim().toLocaleLowerCase(locale);
  const matches = commands.filter((command) =>
    `${t(command.labelKey)} ${command.to}`.toLocaleLowerCase(locale).includes(needle),
  );
  const selectedIndex = Math.min(activeIndex, matches.length - 1);

  const changeOpen = (next: boolean) => {
    if (next) {
      previousFocus.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      navigated.current = false;
      setQuery('');
      setActiveIndex(0);
    }
    setOpen(next);
  };

  useEffect(() => {
    const shortcut = (event: KeyboardEvent) => {
      if (
        event.defaultPrevented ||
        event.isComposing ||
        event.repeat ||
        event.altKey ||
        !(event.ctrlKey || event.metaKey) ||
        event.key.toLowerCase() !== 'k'
      )
        return;
      if (!open && document.querySelector('[role="dialog"][data-state="open"]') !== null) return;
      event.preventDefault();
      if (!open) {
        previousFocus.current =
          document.activeElement instanceof HTMLElement ? document.activeElement : null;
        navigated.current = false;
        setQuery('');
        setActiveIndex(0);
      }
      setOpen(!open);
    };
    window.addEventListener('keydown', shortcut);
    return () => {
      window.removeEventListener('keydown', shortcut);
    };
  }, [open]);

  useEffect(() => {
    activeOption.current?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex, query]);

  const choose = (command: NavigationItem) => {
    navigated.current = true;
    setOpen(false);
    void navigate({
      to: command.to,
      search: selectedSource === undefined ? {} : { source: selectedSource },
    });
  };

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogTrigger asChild>
        <button
          className="app-topbar__notification ui-focus-ring"
          type="button"
          aria-label={t('shell.command.open')}
          aria-keyshortcuts="Control+K Meta+K"
        >
          <Command aria-hidden="true" size={18} />
        </button>
      </DialogTrigger>
      <DialogContent
        className="command-palette"
        title={t('shell.command.title')}
        description={t('shell.command.description')}
        closeLabel={t('shell.command.close')}
        onCloseAutoFocus={(event) => {
          const target = navigated.current
            ? document.getElementById('main-content')
            : previousFocus.current;
          if (target?.isConnected) {
            event.preventDefault();
            target.focus();
          }
        }}
      >
        <Input
          aria-label={t('shell.command.search')}
          role="combobox"
          aria-autocomplete="list"
          aria-expanded={open}
          aria-controls={listId}
          aria-activedescendant={
            selectedIndex < 0 ? undefined : `${listId}-${String(selectedIndex)}`
          }
          autoComplete="off"
          maxLength={200}
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActiveIndex(0);
          }}
          onKeyDown={(event) => {
            if (event.nativeEvent.isComposing) return;
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
              event.preventDefault();
              if (matches.length > 0)
                setActiveIndex(
                  (selectedIndex + (event.key === 'ArrowDown' ? 1 : -1) + matches.length) %
                    matches.length,
                );
            } else if (event.key === 'Enter') {
              event.preventDefault();
              const command = matches[selectedIndex];
              if (command !== undefined) choose(command);
            }
          }}
        />
        <ul
          className="command-palette__results"
          id={listId}
          role="listbox"
          aria-label={t('shell.command.results')}
        >
          {matches.map((command, index) => {
            const Icon = command.icon;
            return (
              <li
                className="command-palette__option ui-focus-ring"
                key={command.to}
                id={`${listId}-${String(index)}`}
                role="option"
                aria-selected={index === selectedIndex}
                tabIndex={-1}
                ref={index === selectedIndex ? activeOption : undefined}
                onMouseDown={(event) => {
                  event.preventDefault();
                }}
                onPointerMove={() => {
                  setActiveIndex(index);
                }}
                onClick={() => {
                  choose(command);
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    choose(command);
                  }
                }}
              >
                <Icon aria-hidden="true" width={18} height={18} />
                <span>{t(command.labelKey)}</span>
                <bdi className="command-palette__path" dir="ltr">
                  {command.to}
                </bdi>
              </li>
            );
          })}
        </ul>
        {matches.length === 0 ? <p role="status">{t('shell.command.empty')}</p> : null}
      </DialogContent>
    </Dialog>
  );
}
