import { useLocale } from '../i18n';
import { useExplorationView } from '../features/product/exploration-ui';
import { WorkspaceNavigation, workspaceHref } from '../features/product/WorkspaceNavigation';
import { useProductSource } from '../features/product/product-context';

const topics = ['start', 'monitoring', 'assistant', 'security'] as const;
export function HelpPage() {
  const { t } = useLocale();
  const view = useExplorationView();
  const { selectedSource } = useProductSource();
  const selected = view.read('help_topic');
  const topic = topics.find((item) => item === selected) ?? 'start';
  return (
    <div className="route-page help-page">
      <header className="route-page__header">
        <h1>{t('route.help.title')}</h1>
        <p>{t('route.help.description')}</p>
      </header>
      <WorkspaceNavigation current="help" />
      <div className="help-layout">
        <nav className="help-topics" aria-label={t('help.topics')}>
          {topics.map((item) => (
            <a
              key={item}
              href={`/help?${new URLSearchParams({ help_topic: item, ...(selectedSource === undefined ? {} : { source: selectedSource }) }).toString()}`}
              aria-current={topic === item ? 'page' : undefined}
            >
              {t(`help.${item}.title`)}
            </a>
          ))}
        </nav>
        <article className="help-article" aria-labelledby="help-topic-title">
          <h2 id="help-topic-title">{t(`help.${topic}.title`)}</h2>
          <p>{t(`help.${topic}.intro`)}</p>
          <ol>
            {(['one', 'two', 'three'] as const).map((step) => (
              <li key={step}>{t(`help.${topic}.${step}`)}</li>
            ))}
          </ol>
          <nav className="resource-detail-links" aria-label={t('help.related')}>
            <a href={workspaceHref('/resources', selectedSource)}>{t('route.resources.title')}</a>
            <a href={workspaceHref('/monitoring', selectedSource)}>{t('route.monitoring.title')}</a>
            <a href={workspaceHref('/assistant', selectedSource)}>{t('route.assistant.title')}</a>
          </nav>
        </article>
      </div>
      <section className="help-support" aria-labelledby="help-support-title">
        <h2 id="help-support-title">{t('help.support.title')}</h2>
        <p>{t('help.support.description')}</p>
        <a
          href="https://github.com/Ilia-Shakeri/Stackplore/issues/new"
          target="_blank"
          rel="noopener noreferrer"
        >
          {t('help.support.action')}
        </a>
      </section>
    </div>
  );
}
