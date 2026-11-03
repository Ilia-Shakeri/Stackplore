import { useLocale } from '../../i18n';
import { useProductSource } from './product-context';

const destinations = [
  { id: 'overview', path: '/overview' },
  { id: 'monitoring', path: '/monitoring' },
  { id: 'insights', path: '/insights' },
  { id: 'assistant', path: '/assistant' },
  { id: 'help', path: '/help' },
] as const;

export type WorkspacePage = (typeof destinations)[number]['id'];

export function workspaceHref(path: string, source?: string) {
  return source === undefined ? path : `${path}?source=${encodeURIComponent(source)}`;
}

export function WorkspaceNavigation({ current }: { readonly current: WorkspacePage }) {
  const { t } = useLocale();
  const { selectedSource } = useProductSource();
  return (
    <nav className="workspace-navigation" aria-label={t('workspace.navigation')}>
      {destinations.map(({ id, path }) => (
        <a
          key={id}
          href={workspaceHref(path, selectedSource)}
          aria-label={t('workspace.open', { name: t(`route.${id}.title`) })}
          aria-current={id === current ? 'page' : undefined}
        >
          {t(`route.${id}.title`)}
        </a>
      ))}
    </nav>
  );
}

export function WorkspaceShortcuts() {
  const { t } = useLocale();
  const { selectedSource } = useProductSource();
  return (
    <section aria-labelledby="workspace-shortcuts-title">
      <h2 id="workspace-shortcuts-title">{t('workspace.choose')}</h2>
      <div className="workspace-shortcuts">
        {destinations
          .filter(({ id }) => id !== 'overview')
          .map(({ id, path }) => (
            <a key={id} href={workspaceHref(path, selectedSource)}>
              <strong>{t(`route.${id}.title`)}</strong>
              <span>{t(`route.${id}.description`)}</span>
            </a>
          ))}
      </div>
    </section>
  );
}
