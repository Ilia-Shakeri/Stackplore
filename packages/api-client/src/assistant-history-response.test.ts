import { describe, expect, it } from 'vitest';
import { assistantReadyFixture, assistantRunId } from './assistant-fixture.test-support';
import { parseAssistantIncidentReportResponse } from './assistant-report-response';

const fixture = () => {
  const base = assistantReadyFixture();
  const selected = base.citations[0];
  if (selected === undefined) throw new Error('Missing selected resource');
  const ref = { ...selected, observed_at: base.run.created_at };
  const at = base.run.scope.range.from;
  const alertRef = { ...ref, id: `ev_${'d'.repeat(64)}`, kind: 'alert' };
  const workloadRef = { ...ref, id: `ev_${'e'.repeat(64)}`, kind: 'workload' };
  const common = {
    at,
    availability: 'available',
    completeness: 'complete',
    synthetic: true,
    health: '',
    metric: null,
    severity: '',
    redacted: false,
    truncated: false,
  };
  return {
    run: { ...base.run, goal: 'investigate_incident' },
    content_state: 'ready',
    citations: [ref, alertRef, workloadRef],
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
      availability: [0, 5, 6].includes(index) ? 'available' : 'unsupported',
      completeness: 'complete',
      records: [0, 5, 6].includes(index) ? 1 : 0,
      more_available: false,
    })),
    report: {
      timeline: [
        {
          ...common,
          citation_id: alertRef.id,
          kind: 'alert',
          time_basis: 'occurred',
          alert: { from: null, to: 'firing' },
        },
        {
          ...common,
          citation_id: workloadRef.id,
          kind: 'workload',
          time_basis: 'sampled',
          workload: { desired: 3, ready: 1 },
        },
        {
          ...common,
          citation_id: ref.id,
          kind: 'resource',
          at: ref.observed_at,
          time_basis: 'observed',
          health: 'healthy',
        },
      ] as const,
      hypotheses: [
        {
          code: 'workload_shortfall_possible',
          rank: 1,
          qualified: true,
          supporting: [workloadRef.id],
          contrary: [],
        },
      ],
      gaps: ['cause_unproven', 'metrics_missing', 'events_missing'],
      next_reads: [
        'review_health',
        'review_metrics',
        'review_events',
        'review_alert_history',
        'review_workload_history',
      ],
    },
  };
};

describe('authorized incident history wire facts', () => {
  it('accepts actual occurrence/sample time and detaches historical values', () => {
    const value = fixture();
    const parsed = parseAssistantIncidentReportResponse(value, assistantRunId);
    expect(parsed).toEqual(value);
    Object.assign(value.report.timeline[1].workload, { ready: 3 });
    if (parsed.content_state !== 'ready') throw new Error('Missing report');
    expect(parsed.report.timeline[1]?.workload?.ready).toBe(1);
  });

  it.each([
    (value: ReturnType<typeof fixture>) => {
      Object.assign(value.report.timeline[0].alert, { from: 'firing' });
    },
    (value: ReturnType<typeof fixture>) => {
      Object.assign(value.report.timeline[1].workload, { ready: -1 });
    },
    (value: ReturnType<typeof fixture>) => {
      Object.assign(value.report.timeline[1].workload, { desired: Number.MAX_SAFE_INTEGER + 1 });
    },
    (value: ReturnType<typeof fixture>) => {
      Object.assign(value.report.timeline[1], { time_basis: 'occurred' });
    },
    (value: ReturnType<typeof fixture>) => {
      Object.assign(value.report.timeline[1], { availability: 'unavailable' });
    },
    (value: ReturnType<typeof fixture>) => {
      value.report.gaps.push('workload_history_not_read');
    },
    (value: ReturnType<typeof fixture>) => {
      const item = value.coverage[6];
      if (item === undefined) throw new Error('Missing history coverage');
      Object.assign(item, { availability: 'unsupported', records: 0 });
    },
    (value: ReturnType<typeof fixture>) => {
      Object.assign(value.report.timeline[2], { alert: { from: null, to: 'firing' } });
    },
  ])('rejects unsafe or contradictory history %#', (mutate) => {
    const value = fixture();
    mutate(value);
    expect(() => parseAssistantIncidentReportResponse(value, assistantRunId)).toThrow(
      'unexpected_response',
    );
  });

  it('distinguishes readable empty history from unread history', () => {
    const value = fixture();
    const empty = {
      ...value,
      citations: value.citations.slice(0, 1),
      coverage: value.coverage.map((item, index) => (index > 4 ? { ...item, records: 0 } : item)),
      report: { ...value.report, timeline: value.report.timeline.slice(2), hypotheses: [] },
    };
    expect(parseAssistantIncidentReportResponse(empty, assistantRunId)).toEqual(empty);
    empty.report.gaps.push('alerts_not_read');
    expect(() => parseAssistantIncidentReportResponse(empty, assistantRunId)).toThrow(
      'unexpected_response',
    );
  });
});
