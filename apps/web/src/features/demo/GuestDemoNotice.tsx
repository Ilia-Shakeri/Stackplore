import { TechnicalText } from '@stackplore/ui';
import { useAuth } from '../auth/auth-context';
import { useLocale } from '../../i18n';
import './demo.css';

export function GuestDemoNotice() {
  const { snapshot } = useAuth();
  const { formatDate, t } = useLocale();
  if (snapshot.status !== 'guest_demo') return null;
  const steps = ['demo.step.initial', 'demo.step.recovering', 'demo.step.healthy'] as const;
  return (
    <aside className="guest-demo-notice" aria-label={t('demo.mode.title')}>
      <div>
        <strong>{t('demo.mode.title')}</strong>
        <span>{t('demo.mode.readOnly')}</span>
      </div>
      <p>{t('demo.mode.description')}</p>
      <dl>
        <div>
          <dt>{t('demo.mode.fixture')}</dt>
          <dd>
            <TechnicalText>{snapshot.fixtureVersion}</TechnicalText>
          </dd>
        </div>
        <div>
          <dt>{t('demo.mode.step')}</dt>
          <dd>{t(steps[snapshot.scenarioStep])}</dd>
        </div>
        <div>
          <dt>{t('demo.mode.clock')}</dt>
          <dd>
            <time dateTime={snapshot.observedAt}>
              {formatDate(Date.parse(snapshot.observedAt), {
                dateStyle: 'medium',
                timeStyle: 'short',
              })}
            </time>
          </dd>
        </div>
      </dl>
    </aside>
  );
}
