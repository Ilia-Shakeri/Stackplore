import type { ProductLogRecord } from '@stackplore/api-client';
import type { TextMessageKey, useLocale } from '../../i18n';

const severityKeys = {
  trace: 'explore.logs.severity.trace',
  debug: 'explore.events.severity.debug',
  info: 'explore.events.severity.info',
  warning: 'explore.events.severity.warning',
  error: 'explore.events.severity.error',
  critical: 'explore.events.severity.critical',
  unknown: 'explore.logs.severity.unknown',
} as const satisfies Record<ProductLogRecord['severity'], TextMessageKey>;

export function severityLabel(
  severity: ProductLogRecord['severity'],
  t: ReturnType<typeof useLocale>['t'],
) {
  return t(severityKeys[severity]);
}
