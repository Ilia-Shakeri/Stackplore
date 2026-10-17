import { useLocale } from '../../i18n';
import type { CleanupImpact } from './settings-types';

export function SettingsImpact({ impact }: { readonly impact: CleanupImpact }) {
  const { t, formatDate, formatNumber } = useLocale();
  return (
    <section className="settings-impact" aria-label={t('settings.dialog.history')}>
      {impact.classes.length > 0 ? (
        <div className="settings-table-scroll">
          <table>
            <caption>{t('settings.dialog.history')}</caption>
            <thead>
              <tr>
                <th scope="col">{t('settings.dialog.field')}</th>
                <th scope="col">{t('settings.dialog.cutoff')}</th>
                <th scope="col">{t('settings.dialog.count')}</th>
              </tr>
            </thead>
            <tbody>
              {impact.classes.map((item) => (
                <tr key={item.name}>
                  <th scope="row">{t(`settings.cleanup.${item.name}`)}</th>
                  <td>
                    <time dateTime={item.cutoff}>
                      {formatDate(Date.parse(item.cutoff), {
                        dateStyle: 'medium',
                        timeStyle: 'medium',
                        timeZone: 'UTC',
                      })}
                    </time>
                  </td>
                  <td>
                    {item.eligible_rows_capped
                      ? t('settings.dialog.countCapped', { count: item.eligible_rows })
                      : formatNumber(item.eligible_rows)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
      {impact.classes.length > 0 ? (
        <p>{t('settings.dialog.batch', { count: impact.maximum_rows })}</p>
      ) : null}
      {impact.expiry_policy_changed ? (
        <p className="settings-impact__warning">{t('settings.dialog.continuous')}</p>
      ) : null}
      {impact.backup_required ? (
        <p className="settings-impact__warning">{t('settings.dialog.backup')}</p>
      ) : null}
    </section>
  );
}
