import {
  parseAssistantRunResponse,
  parseAssistantIncidentReportResponse,
  type AssistantRun,
} from '@stackplore/api-client';
import { vi } from 'vitest';

import type { AssistantApi } from './assistant-api';

export const scope = {
  sourceId: 'src_demo_01',
  resourceId: `r1_${'A'.repeat(43)}`,
  windowMinutes: 15 as const,
  locale: 'en' as const,
};
export const resourceRun = (
  state: AssistantRun['state'] = 'completed',
  version = 3,
): AssistantRun => ({
  id: `ar_${'a'.repeat(64)}`,
  conversation_id: `ac_${'c'.repeat(64)}`,
  goal: 'explain_resource',
  state,
  version,
  scope: {
    source_id: scope.sourceId,
    resource_id: scope.resourceId,
    window_minutes: scope.windowMinutes,
    range: { from: '2026-10-28T10:00:00Z', to: '2026-10-28T10:15:00Z' },
  },
  created_at: '2026-10-28T10:15:00Z',
  updated_at: version === 1 ? '2026-10-28T10:15:00Z' : '2026-10-28T10:15:01Z',
  deadline: '2026-10-28T10:17:00Z',
  usage_uncertain: false,
  usage_conservative: false,
});
export const readyResponse = () => {
  const run = resourceRun();
  const citation = {
    id: `ev_${'b'.repeat(64)}`,
    kind: 'resource',
    source_id: scope.sourceId,
    resource_id: scope.resourceId,
    observed_at: run.created_at,
    range: run.scope.range,
  };
  return parseAssistantRunResponse(
    {
      run,
      content_state: 'ready',
      answer: {
        sections: [
          { kind: 'facts', text: 'Selected resource is available.', citations: [citation.id] },
        ],
      },
      citations: [citation],
    },
    run.id,
  );
};
export const lostResponse = (run = resourceRun()) =>
  parseAssistantRunResponse({ run, content_state: 'volatile_lost' }, run.id);
export const investigationResponse = () => {
  const ready = readyResponse();
  if (ready.content_state !== 'ready') throw new Error('Missing investigation fixture');
  return parseAssistantRunResponse(
    {
      ...ready,
      run: { ...ready.run, goal: 'investigate_incident' },
      coverage: [
        'inventory',
        'health',
        'metrics',
        'events',
        'topology',
        'alert_history',
        'workload_history',
      ].map((tool, index) => ({
        tool,
        availability: index === 0 ? 'available' : 'unsupported',
        completeness: 'complete',
        records: index === 0 ? 1 : 0,
        more_available: false,
      })),
    },
    ready.run.id,
  );
};
export const incidentResponse = () => {
  const ready = investigationResponse();
  if (ready.content_state !== 'ready') throw new Error('Missing report answer fixture');
  const ref = ready.citations[0];
  return parseAssistantIncidentReportResponse(
    {
      run: ready.run,
      content_state: 'ready',
      citations: ready.citations,
      coverage: ready.coverage,
      report: {
        timeline: [
          {
            citation_id: ref.id,
            kind: ref.kind,
            at: ref.observed_at,
            time_basis: 'observed',
            availability: 'available',
            completeness: 'complete',
            synthetic: true,
            health: 'healthy',
            metric: null,
            severity: '',
            redacted: false,
            truncated: false,
          },
        ],
        hypotheses: [],
        gaps: [
          'alerts_not_read',
          'workload_history_not_read',
          'cause_unproven',
          'metrics_missing',
          'events_missing',
        ],
        next_reads: [
          'review_health',
          'review_metrics',
          'review_events',
          'review_alert_history',
          'review_workload_history',
        ],
      },
    },
    ready.run.id,
  );
};

export const historicalResponses = () => {
  const base = investigationResponse();
  const reportBase = incidentResponse();
  if (base.content_state !== 'ready' || reportBase.content_state !== 'ready')
    throw new Error('Missing history report fixture');
  const selected = base.citations[0];
  const alert = { ...selected, id: `ev_${'d'.repeat(64)}`, kind: 'alert' };
  const workload = { ...selected, id: `ev_${'e'.repeat(64)}`, kind: 'workload' };
  const citations = [...base.citations, alert, workload];
  const coverage = base.coverage?.map((item) =>
    item.tool === 'alert_history' || item.tool === 'workload_history'
      ? { ...item, availability: 'available', records: 1 }
      : item,
  );
  const ready = parseAssistantRunResponse({ ...base, citations, coverage }, base.run.id);
  const common = {
    at: base.run.scope.range.from,
    availability: 'available',
    completeness: 'complete',
    synthetic: true,
    health: '',
    metric: null,
    severity: '',
    redacted: false,
    truncated: false,
  };
  const report = parseAssistantIncidentReportResponse(
    {
      ...reportBase,
      citations,
      coverage,
      report: {
        ...reportBase.report,
        timeline: [
          {
            ...common,
            citation_id: alert.id,
            kind: 'alert',
            time_basis: 'occurred',
            alert: { from: null, to: 'firing' },
          },
          {
            ...common,
            citation_id: workload.id,
            kind: 'workload',
            time_basis: 'sampled',
            workload: { desired: 3, ready: 1 },
          },
          ...reportBase.report.timeline,
        ],
        hypotheses: [
          {
            code: 'workload_shortfall_possible',
            rank: 1,
            qualified: true,
            supporting: [workload.id],
            contrary: [],
          },
        ],
        gaps: reportBase.report.gaps.filter(
          (gap) => gap !== 'alerts_not_read' && gap !== 'workload_history_not_read',
        ),
      },
    },
    base.run.id,
  );
  return { ready, report, workload };
};
export const pendingResponse = () => {
  const run = resourceRun('queued', 1);
  return parseAssistantRunResponse({ run, content_state: 'pending' }, run.id);
};
export function controlledConversationApi() {
  let owner: symbol | null = Symbol();
  const listeners = new Set<() => void>();
  const api = {
    history: vi.fn<AssistantApi['history']>().mockResolvedValue({ entries: [], next_cursor: null }),
    getOwner: () => owner,
    subscribeOwner(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    admit: vi.fn<AssistantApi['admit']>().mockResolvedValue({ run: resourceRun() }),
    get: vi.fn<AssistantApi['get']>().mockResolvedValue(lostResponse()),
    report: vi
      .fn<AssistantApi['report']>()
      .mockRejectedValue(new Error('Unexpected synthetic report call')),
    cancel: vi
      .fn<AssistantApi['cancel']>()
      .mockResolvedValue({ run: { ...resourceRun(), state: 'cancelled', version: 4 } }),
    citation: vi
      .fn<AssistantApi['citation']>()
      .mockRejectedValue(new Error('Unexpected synthetic citation call')),
    status: vi.fn<AssistantApi['status']>().mockResolvedValue('ended'),
  } satisfies AssistantApi;
  return {
    api,
    listeners,
    setOwner: (next: symbol | null) => {
      owner = next;
      for (const listener of listeners) listener();
    },
  };
}
