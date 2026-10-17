import type { ProductAlert } from '@stackplore/api-client';
import type { useLocale, TextMessageKey } from '../../i18n';

export function alertStateLabel(
  state: ProductAlert['state'],
  t: ReturnType<typeof useLocale>['t'],
) {
  switch (state) {
    case 'pending':
      return t('product.alert.pending');
    case 'firing':
      return t('product.alert.firing');
    case 'acknowledged':
      return t('product.alert.acknowledged');
    case 'resolved':
      return t('product.alert.resolved');
  }
}

export function alertSummary(code: string, t: ReturnType<typeof useLocale>['t']) {
  const messages: Readonly<Record<string, TextMessageKey>> = {
    'demo.alert.checkout_unhealthy': 'product.alert.checkoutUnhealthy',
    'alert.resource_unavailable.matched': 'product.alert.resourceUnavailable',
    'alert.resource_unavailable.clear': 'product.alert.resourceUnavailableCleared',
    'alert.resource_unhealthy.matched': 'product.alert.resourceUnhealthy',
    'alert.resource_unhealthy.clear': 'product.alert.resourceUnhealthyCleared',
    'alert.restart_threshold.matched': 'product.alert.restartThreshold',
    'alert.restart_threshold.clear': 'product.alert.restartThresholdCleared',
    'alert.cpu_threshold.matched': 'product.alert.cpuThreshold',
    'alert.cpu_threshold.clear': 'product.alert.cpuThresholdCleared',
    'alert.memory_threshold.matched': 'product.alert.memoryThreshold',
    'alert.memory_threshold.clear': 'product.alert.memoryThresholdCleared',
    'alert.workload_unavailable_replicas.matched': 'product.alert.workloadUnavailableReplicas',
    'alert.workload_unavailable_replicas.clear': 'product.alert.workloadUnavailableReplicasCleared',
    'alert.event_severity_match.matched': 'product.alert.eventSeverity',
    'alert.event_severity_match.clear': 'product.alert.eventSeverityCleared',
    'alert.provider_connection_lost.matched': 'product.alert.providerConnectionLost',
    'alert.provider_connection_lost.clear': 'product.alert.providerConnectionLostCleared',
  };
  return t(
    Object.hasOwn(messages, code)
      ? (messages[code] ?? 'product.alert.unknown')
      : 'product.alert.unknown',
  );
}
