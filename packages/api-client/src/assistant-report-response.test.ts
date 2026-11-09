import { describe, expect, it, vi } from 'vitest';
import { assistantReadyFixture, assistantRunId } from './assistant-fixture.test-support';
import { parseAssistantIncidentReportResponse } from './assistant-report-response';

export const incidentReportFixture = () => {
  const base = assistantReadyFixture();
  const selected = base.citations[0];
  if (selected === undefined) throw new Error('Missing report resource fixture');
  const ref = { ...selected, observed_at: base.run.created_at };
  return {
    run: { ...base.run, goal: 'investigate_incident' },
    content_state: 'ready',
    citations: [ref],
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
    report: {
      timeline: [
        {
          citation_id: ref.id,
          kind: 'resource',
          at: ref.observed_at,
          time_basis: 'observed',
          availability: 'available',
          completeness: 'complete',
          synthetic: true,
          health: 'healthy',
          metric: null as null | {
            name: string;
            unit: string;
            value: number;
            interval_seconds: number;
          },
          severity: '',
          redacted: false,
          truncated: false,
        },
      ],
      hypotheses: [] as {
        code: string;
        rank: number;
        qualified: boolean;
        supporting: string[];
        contrary: string[];
      }[],
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
  };
};

const pressureFixture = () => {
  const value = incidentReportFixture();
  const original = value.citations[0],
    inventory = value.report.timeline[0],
    metricCoverage = value.coverage[2];
  if (original === undefined || inventory === undefined || metricCoverage === undefined)
    throw new Error('Missing pressure fixture');
  const at = value.run.scope.range.from;
  const usageId = `ev_${'1'.repeat(64)}`,
    limitId = `ev_${'2'.repeat(64)}`;
  value.citations.push(
    { ...original, id: usageId, kind: 'metric' },
    { ...original, id: limitId, kind: 'metric' },
  );
  value.report.timeline = [
    {
      ...inventory,
      citation_id: usageId,
      kind: 'metric',
      at,
      time_basis: 'sampled',
      health: '',
      metric: { name: 'memory.usage.bytes', unit: 'bytes', value: 1024, interval_seconds: 1 },
    },
    {
      ...inventory,
      citation_id: limitId,
      kind: 'metric',
      at,
      time_basis: 'sampled',
      health: '',
      metric: { name: 'memory.limit.bytes', unit: 'bytes', value: 512, interval_seconds: 1 },
    },
    inventory,
  ];
  metricCoverage.availability = 'available';
  metricCoverage.records = 2;
  value.report.gaps = value.report.gaps.filter((code) => code !== 'metrics_missing');
  value.report.hypotheses.push({
    code: 'memory_pressure_possible',
    rank: 1,
    qualified: true,
    supporting: [usageId, limitId],
    contrary: [],
  });
  return value;
};

describe('owning typed incident report response', () => {
  it('retains the guide witness but rejects it as an observed timeline row', () => {
    const base = incidentReportFixture();
    const resource = base.citations[0];
    if (resource === undefined) throw new Error('Missing fixture');
    const guide = {
      ...resource,
      id: `ev_${'e'.repeat(64)}`,
      kind: 'document',
      document_reference: {
        corpus_id: 'ops',
        version: 2,
        digest: 'a'.repeat(64),
        document_id: 'guide',
        locale: 'en',
        generation: 0,
      },
    };
    const ready = {
      ...base,
      citations: [...base.citations, guide],
      coverage: [...base.coverage, { tool: 'reviewed_document', availability: 'available', completeness: 'complete', records: 1, more_available: false }],
    };
    const parsed = parseAssistantIncidentReportResponse(ready, assistantRunId);
    expect(parsed.content_state).toBe('ready');
    if (parsed.content_state !== 'ready') throw new Error('Expected ready report');
    expect(parsed.report.timeline).toHaveLength(1);
    expect(parsed.citations).toHaveLength(2);
    expect(() => parseAssistantIncidentReportResponse({
      ...ready,
      report: { ...ready.report, timeline: [{ ...ready.report.timeline[0], citation_id: guide.id, kind: 'document' }] },
    }, assistantRunId)).toThrow();
  });
  it('accepts actual typed chronology and detaches nested metric/support arrays', () => {
    const value = pressureFixture();
    const parsed = parseAssistantIncidentReportResponse(value, assistantRunId);
    expect(parsed).toEqual(value);
    if (parsed.content_state !== 'ready') throw new Error('Missing ready report');
    const metric = value.report.timeline[0]?.metric,
      support = value.report.hypotheses[0]?.supporting;
    if (metric === null || metric === undefined || support === undefined)
      throw new Error('Missing pressure inputs');
    metric.value = 1;
    support[0] = 'changed';
    value.report.gaps[0] = 'changed';
    expect(parsed.report.timeline[0]?.metric?.value).toBe(1024);
    expect(parsed.report.hypotheses[0]?.supporting[0]).not.toBe('changed');
    expect(parsed.report.gaps[0]).toBe('alerts_not_read');
  });

  it.each([
    (value: ReturnType<typeof pressureFixture>) => {
      value.run.goal = 'explain_resource';
    },
    (value: ReturnType<typeof pressureFixture>) => {
      value.report.next_reads[0] = 'run_shell';
    },
    (value: ReturnType<typeof pressureFixture>) => {
      value.report.gaps = [];
    },
    (value: ReturnType<typeof pressureFixture>) => {
      value.report.timeline.reverse();
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.timeline[0];
      if (item !== undefined) item.at = value.run.scope.range.to;
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.timeline[0];
      if (item !== undefined) item.citation_id = `ev_${'f'.repeat(64)}`;
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.timeline[0];
      if (item !== undefined) item.kind = 'log';
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.timeline[0]?.metric;
      if (item !== null && item !== undefined) item.value = Infinity;
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.timeline[0]?.metric;
      if (item !== null && item !== undefined) item.unit = 'count';
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.timeline[1]?.metric;
      if (item !== null && item !== undefined) item.value = 0;
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.hypotheses[0];
      if (item !== undefined) item.qualified = false;
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.hypotheses[0];
      if (item !== undefined) item.code = 'oom_proven';
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.hypotheses[0];
      if (item !== undefined) item.rank = 2;
    },
    (value: ReturnType<typeof pressureFixture>) => {
      const item = value.report.hypotheses[0];
      if (item !== undefined)
        item.supporting = [item.supporting[0] ?? '', item.supporting[0] ?? ''];
    },
  ])('rejects forged scope, chronology, scalar facts or causal claims %#', (change) => {
    const value = pressureFixture();
    change(value);
    expect(() => parseAssistantIncidentReportResponse(value, assistantRunId)).toThrow(
      'unexpected_response',
    );
  });

  it('never invokes rejected getters or coercion hooks', () => {
    const value = pressureFixture();
    const hook = vi.fn(() => 'memory.usage.bytes');
    const metric = value.report.timeline[0]?.metric;
    if (metric === null || metric === undefined) throw new Error('Missing metric');
    Object.defineProperty(metric, 'name', { value: { toString: hook }, enumerable: true });
    expect(() => parseAssistantIncidentReportResponse(value, assistantRunId)).toThrow(
      'unexpected_response',
    );
    expect(hook).not.toHaveBeenCalled();
    const other = incidentReportFixture();
    Object.defineProperty(other.report, 'timeline', { get: hook, enumerable: true });
    expect(() => parseAssistantIncidentReportResponse(other, assistantRunId)).toThrow(
      'unexpected_response',
    );
    expect(hook).not.toHaveBeenCalled();
  });

  it('keeps lost memory metadata-only and rejects mismatched run targets', () => {
    const value = incidentReportFixture();
    expect(
      parseAssistantIncidentReportResponse(
        { run: value.run, content_state: 'volatile_lost' },
        assistantRunId,
      ).content_state,
    ).toBe('volatile_lost');
    expect(() =>
      parseAssistantIncidentReportResponse(
        { run: value.run, content_state: 'volatile_lost', report: value.report },
        assistantRunId,
      ),
    ).toThrow('unexpected_response');
    expect(() => parseAssistantIncidentReportResponse(value, `ar_${'f'.repeat(64)}`)).toThrow(
      'unexpected_response',
    );
  });
});
