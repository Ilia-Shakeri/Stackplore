import type { AssistantIncidentReportReadyResponse } from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { useEffect, useRef, useState } from 'react';
import { useLocale, type MessageKey } from '../../i18n';
import type { AssistantConversation } from './conversation';

const families = {
  resource: 'assistant.tool.inventory',
  health: 'assistant.tool.health',
  metric: 'assistant.tool.metrics',
  event: 'assistant.tool.events',
  relationship: 'assistant.tool.topology',
  log: 'assistant.tool.logs',
  alert: 'assistant.tool.alertHistory',
  workload: 'assistant.tool.workloadHistory',
} as const satisfies Record<string, MessageKey>;
const bases = {
  observed: 'assistant.report.observed',
  sampled: 'assistant.report.sampled',
  occurred: 'assistant.report.occurred',
} as const satisfies Record<string, MessageKey>;
const hypotheses = {
  memory_pressure_possible: 'assistant.report.memory',
  cpu_pressure_possible: 'assistant.report.cpu',
  restart_related_possible: 'assistant.report.restart',
  health_problem_possible: 'assistant.report.health',
  workload_shortfall_possible: 'assistant.report.workload',
} as const satisfies Record<string, MessageKey>;
const alertStates = {
  pending: 'assistant.report.alert.pending',
  firing: 'assistant.report.alert.firing',
  acknowledged: 'assistant.report.alert.acknowledged',
  resolved: 'assistant.report.alert.resolved',
} as const satisfies Record<string, MessageKey>;
const availability = {
  available: 'assistant.coverage.available',
  stale: 'assistant.coverage.stale',
  unavailable: 'assistant.coverage.unavailable',
  unsupported: 'assistant.coverage.unsupported',
} as const satisfies Record<string, MessageKey>;
const health = {
  healthy: 'assistant.report.healthy',
  degraded: 'assistant.report.degraded',
  unhealthy: 'assistant.report.unhealthy',
  unknown: 'assistant.report.unknown',
} as const satisfies Record<string, MessageKey>;
const gaps = {
  alerts_not_read: 'assistant.report.gap.alerts',
  workload_history_not_read: 'assistant.report.gap.workloads',
  cause_unproven: 'assistant.report.gap.cause',
  limited_evidence: 'assistant.report.gap.limited',
  matching_limits_missing: 'assistant.report.gap.limits',
  metrics_missing: 'assistant.report.gap.metrics',
  events_missing: 'assistant.report.gap.events',
  conflicting_samples: 'assistant.report.gap.samples',
  restart_counter_reset: 'assistant.report.gap.reset',
  conflicting_health: 'assistant.report.gap.health',
  out_of_window_observation: 'assistant.report.gap.window',
} as const satisfies Record<string, MessageKey>;
const reads = {
  review_health: 'assistant.report.read.health',
  review_metrics: 'assistant.report.read.metrics',
  review_events: 'assistant.report.read.events',
  review_alert_history: 'assistant.report.read.alerts',
  review_workload_history: 'assistant.report.read.workloads',
} as const satisfies Record<string, MessageKey>;
type View = {
  readonly context: AssistantConversation;
  readonly runId: string;
  readonly state: 'loading' | 'ready' | 'unavailable';
  readonly response?: AssistantIncidentReportReadyResponse;
};

export function IncidentReportView({
  conversation,
  runId,
  onCitation,
}: {
  readonly conversation: AssistantConversation;
  readonly runId: string;
  readonly onCitation: (id: string) => void;
}) {
  const { t, formatNumber } = useLocale();
  const [view, setView] = useState<View | null>(null);
  const work = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      work.current?.abort();
      work.current = null;
    },
    [conversation, runId],
  );
  const visible = view?.context === conversation && view.runId === runId ? view : null;
  const response = visible?.state === 'ready' ? visible.response : undefined;
  const report = response?.report;
  const references = (ids: readonly string[]) =>
    ids.map((id, index) => (
      <Button
        key={id}
        variant="outline"
        onClick={() => {
          onCitation(id);
        }}
      >
        {t('assistant.citation')} {formatNumber(index + 1)}
      </Button>
    ));
  return (
    <section className="assistant__report" aria-label={t('assistant.report.title')}>
      <Button
        variant="outline"
        aria-expanded={visible !== null}
        onClick={() => {
          work.current?.abort();
          work.current = null;
          if (visible !== null) {
            setView(null);
            return;
          }
          const controller = new AbortController();
          work.current = controller;
          setView({ context: conversation, runId, state: 'loading' });
          void conversation.resolveReport(runId, controller.signal).then((response) => {
            if (controller.signal.aborted || work.current !== controller) return;
            work.current = null;
            setView(
              response === null
                ? { context: conversation, runId, state: 'unavailable' }
                : { context: conversation, runId, state: 'ready', response },
            );
          });
        }}
      >
        {t(visible === null ? 'assistant.report.open' : 'assistant.report.close')}
      </Button>
      {visible?.state === 'loading' ? <p role="status">{t('assistant.report.loading')}</p> : null}
      {visible?.state === 'unavailable' ? (
        <p role="status">{t('assistant.report.unavailable')}</p>
      ) : null}
      {report !== undefined ? (
        <>
          <h3>{t('assistant.report.title')}</h3>
          <p>{t('assistant.report.notice')}</p>
          <h4>{t('assistant.report.timeline')}</h4>
          <div
            className="assistant__report-table"
            role="region"
            aria-label={t('assistant.report.timeline')}
            tabIndex={0}
          >
            <table>
              <caption>{t('assistant.report.timeline')}</caption>
              <thead>
                <tr>
                  <th scope="col">{t('assistant.report.time')}</th>
                  <th scope="col">{t('assistant.report.record')}</th>
                  <th scope="col">{t('assistant.report.observation')}</th>
                  <th scope="col">{t('assistant.report.quality')}</th>
                  <th scope="col">{t('assistant.citation')}</th>
                </tr>
              </thead>
              <tbody>
                {report.timeline.map((item) => {
                  const reference = response?.citations.find((ref) => ref.id === item.citation_id);
                  return (
                    <tr
                      key={item.citation_id}
                      id={`incident-evidence-${item.citation_id}`}
                      tabIndex={-1}
                    >
                      <td>
                        {t(bases[item.time_basis])}
                        <br />
                        <time dateTime={item.at}>
                          <TechnicalText>{item.at}</TechnicalText>
                        </time>
                        {item.time_basis !== 'observed' && reference !== undefined ? (
                          <>
                            <br />
                            {t('assistant.report.observed')}
                            <br />
                            <time dateTime={reference.observed_at}>
                              <TechnicalText>{reference.observed_at}</TechnicalText>
                            </time>
                          </>
                        ) : null}
                      </td>
                      <td>{t(families[item.kind])}</td>
                      <td>
                        {item.alert !== undefined ? (
                          <>
                            {item.alert.from === null ? '—' : t(alertStates[item.alert.from])} →{' '}
                            {t(alertStates[item.alert.to])}
                          </>
                        ) : item.workload !== undefined ? (
                          <>
                            {t('assistant.report.replicas')}:{' '}
                            {item.workload.ready === null ? '—' : formatNumber(item.workload.ready)}{' '}
                            /{' '}
                            {item.workload.desired === null
                              ? '—'
                              : formatNumber(item.workload.desired)}
                          </>
                        ) : item.health !== '' ? (
                          t(health[item.health])
                        ) : item.metric !== null ? (
                          <TechnicalText>
                            {item.metric.name}: {formatNumber(item.metric.value)} {item.metric.unit}
                            ; {formatNumber(item.metric.interval_seconds)} s
                          </TechnicalText>
                        ) : item.severity !== '' ? (
                          <TechnicalText>{item.severity}</TechnicalText>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td>
                        {t(availability[item.availability])}
                        {item.completeness === 'partial'
                          ? `; ${t('assistant.coverage.partial')}`
                          : ''}
                        {item.synthetic ? `; ${t('assistant.report.synthetic')}` : ''}
                        {item.redacted ? `; ${t('assistant.report.redacted')}` : ''}
                        {item.truncated ? `; ${t('assistant.report.truncated')}` : ''}
                      </td>
                      <td>{references([item.citation_id])}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <h4>{t('assistant.report.possibilities')}</h4>
          {report.hypotheses.length === 0 ? (
            <p>{t('assistant.report.none')}</p>
          ) : (
            <ol>
              {report.hypotheses.map((item) => (
                <li key={item.code}>
                  <p>{t(hypotheses[item.code])}</p>
                  <p>{t('assistant.report.supporting')}</p>
                  <div className="assistant__actions">{references(item.supporting)}</div>
                  {item.contrary.length > 0 ? (
                    <>
                      <p>{t('assistant.report.contrary')}</p>
                      <div className="assistant__actions">{references(item.contrary)}</div>
                    </>
                  ) : null}
                </li>
              ))}
            </ol>
          )}
          <h4>{t('assistant.report.gaps')}</h4>
          <ul>
            {report.gaps.map((code) => (
              <li key={code}>{t(gaps[code])}</li>
            ))}
          </ul>
          <h4>{t('assistant.report.nextReads')}</h4>
          <ul>
            {report.next_reads.map((code) => (
              <li key={code}>{t(reads[code])}</li>
            ))}
          </ul>
        </>
      ) : null}
    </section>
  );
}
