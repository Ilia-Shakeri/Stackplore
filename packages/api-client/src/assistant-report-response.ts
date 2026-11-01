import { assistantResponseWire } from './assistant-response';
import type {
  AssistantIncidentObservation,
  AssistantIncidentReportResponse,
  AssistantIncidentHypothesis,
  AssistantResourceCitation,
  AssistantRun,
} from './contract';

const { closed, boundedArray, timestamp, parseRun, parseCitation, parseCoverage, fail } =
  assistantResponseWire;
const hypothesisCodes = [
  'memory_pressure_possible',
  'cpu_pressure_possible',
  'restart_related_possible',
  'health_problem_possible',
] as const;
const gapCodes = [
  'alerts_not_read',
  'workload_history_not_read',
  'cause_unproven',
  'limited_evidence',
  'matching_limits_missing',
  'metrics_missing',
  'events_missing',
  'conflicting_samples',
  'restart_counter_reset',
  'conflicting_health',
  'out_of_window_observation',
] as const;
const nextReadCodes = [
  'review_health',
  'review_metrics',
  'review_events',
  'review_alert_history',
  'review_workload_history',
] as const;
const metricNames = [
  'cpu.usage.cores',
  'memory.usage.bytes',
  'restart.count',
  'network.receive.bytes_total',
  'network.send.bytes_total',
  'cpu.request.cores',
  'cpu.limit.cores',
  'memory.request.bytes',
  'memory.limit.bytes',
] as const;
const member = <T extends string>(value: unknown, values: readonly T[]): value is T =>
  typeof value === 'string' && values.includes(value as T);
const codes = (value: unknown, allowed: readonly string[], minimum: number, maximum: number) => {
  const result = boundedArray(value, minimum, maximum);
  if (new Set(result).size !== result.length || result.some((item) => !member(item, allowed)))
    return fail();
  return result as string[];
};

const observation = (
  value: unknown,
  references: Map<string, AssistantResourceCitation>,
  run: AssistantRun,
): AssistantIncidentObservation => {
  const item = closed(value, [
    'citation_id',
    'kind',
    'at',
    'time_basis',
    'availability',
    'completeness',
    'synthetic',
    'health',
    'metric',
    'severity',
    'redacted',
    'truncated',
  ]);
  if (typeof item.citation_id !== 'string') return fail();
  const ref = references.get(item.citation_id);
  if (
    ref === undefined ||
    item.kind !== ref.kind ||
    !member(item.availability, ['available', 'stale']) ||
    !member(item.completeness, ['complete', 'partial']) ||
    typeof item.synthetic !== 'boolean' ||
    typeof item.redacted !== 'boolean' ||
    typeof item.truncated !== 'boolean'
  )
    return fail();
  const observed =
    item.kind === 'resource' || item.kind === 'health' || item.kind === 'relationship';
  const at = timestamp(item.at, true),
    proofAt = timestamp(ref.observed_at);
  const basis = observed ? 'observed' : item.kind === 'metric' ? 'sampled' : 'occurred';
  if (
    item.time_basis !== basis ||
    at > proofAt ||
    proofAt > timestamp(run.updated_at, true) ||
    (observed
      ? at !== proofAt
      : at < timestamp(run.scope.range.from, true) || at >= timestamp(run.scope.range.to, true))
  )
    return fail();
  if (item.kind === 'resource' || item.kind === 'health') {
    if (!member(item.health, ['healthy', 'degraded', 'unhealthy', 'unknown'])) return fail();
  } else if (item.health !== '') return fail();
  let metric: AssistantIncidentObservation['metric'] = null;
  if (item.kind === 'metric') {
    const parsed = closed(item.metric, ['name', 'unit', 'value', 'interval_seconds']);
    if (
      !member(parsed.name, metricNames) ||
      typeof parsed.value !== 'number' ||
      !Number.isFinite(parsed.value) ||
      parsed.value < 0 ||
      parsed.value > Number.MAX_SAFE_INTEGER ||
      typeof parsed.interval_seconds !== 'number' ||
      !Number.isInteger(parsed.interval_seconds) ||
      parsed.interval_seconds < 0 ||
      parsed.interval_seconds > 86400
    )
      return fail();
    const unit = parsed.name.endsWith('.cores')
      ? 'cores'
      : parsed.name === 'restart.count'
        ? 'count'
        : 'bytes';
    if (parsed.unit !== unit || (unit !== 'cores' && !Number.isInteger(parsed.value)))
      return fail();
    metric = {
      name: parsed.name,
      unit,
      value: parsed.value,
      interval_seconds: parsed.interval_seconds,
    };
  } else if (item.metric !== null) return fail();
  if (item.kind === 'event') {
    if (!member(item.severity, ['debug', 'info', 'warning', 'error', 'critical'])) return fail();
  } else if (item.kind === 'log') {
    if (
      !member(item.severity, [
        'trace',
        'debug',
        'info',
        'warning',
        'error',
        'critical',
        'unknown',
      ]) ||
      run.scope.window_minutes > 15
    )
      return fail();
  } else if (item.severity !== '') return fail();
  if (item.kind !== 'log' && (item.redacted || item.truncated)) return fail();
  return { ...item, metric } as AssistantIncidentObservation;
};

const validateHypothesis = (
  item: AssistantIncidentHypothesis,
  timeline: Map<string, AssistantIncidentObservation>,
  run: AssistantRun,
) => {
  const support = item.supporting.map((id) => timeline.get(id));
  const contrary = item.contrary.map((id) => timeline.get(id));
  const actual = [...support, ...contrary];
  if (
    actual.some(
      (fact) =>
        fact === undefined || fact.availability !== 'available' || fact.completeness !== 'complete',
    )
  )
    return fail();
  const first = support[0],
    second = support[1];
  if (first === undefined) return fail();
  if (item.code === 'health_problem_possible') {
    const last = contrary[0];
    if (
      support.length !== 1 ||
      contrary.length > 1 ||
      !['resource', 'health'].includes(first.kind) ||
      !['unhealthy', 'degraded'].includes(first.health) ||
      timestamp(first.at, true) < timestamp(run.scope.range.from, true) ||
      (last !== undefined &&
        (!['resource', 'health'].includes(last.kind) ||
          last.health !== 'healthy' ||
          timestamp(last.at, true) < timestamp(first.at, true)))
    )
      return fail();
    return;
  }
  if (
    second === undefined ||
    support.length !== 2 ||
    first.metric === null ||
    second.metric === null ||
    (contrary.length !== 0 && contrary.length !== 2)
  )
    return fail();
  const third = contrary[0],
    fourth = contrary[1];
  if (item.code === 'restart_related_possible') {
    if (
      first.metric.name !== 'restart.count' ||
      second.metric.name !== 'restart.count' ||
      timestamp(first.at, true) >= timestamp(second.at, true) ||
      first.metric.value >= second.metric.value ||
      (third !== undefined &&
        fourth !== undefined &&
        (third.metric === null ||
          fourth.metric === null ||
          third.metric.name !== 'restart.count' ||
          fourth.metric.name !== 'restart.count' ||
          timestamp(third.at, true) < timestamp(second.at, true) ||
          timestamp(fourth.at, true) <= timestamp(third.at, true) ||
          third.metric.value !== fourth.metric.value))
    )
      return fail();
    return;
  }
  const memory = item.code === 'memory_pressure_possible';
  const usage = memory ? 'memory.usage.bytes' : 'cpu.usage.cores';
  const limit = memory ? 'memory.limit.bytes' : 'cpu.limit.cores';
  if (
    first.metric.name !== usage ||
    second.metric.name !== limit ||
    timestamp(first.at, true) !== timestamp(second.at, true) ||
    first.metric.interval_seconds !== second.metric.interval_seconds ||
    second.metric.value <= 0 ||
    first.metric.value < second.metric.value ||
    (third !== undefined &&
      fourth !== undefined &&
      (third.metric === null ||
        fourth.metric === null ||
        third.metric.name !== usage ||
        fourth.metric.name !== limit ||
        timestamp(third.at, true) < timestamp(first.at, true) ||
        timestamp(third.at, true) !== timestamp(fourth.at, true) ||
        third.metric.interval_seconds !== fourth.metric.interval_seconds ||
        fourth.metric.value <= 0 ||
        third.metric.value >= fourth.metric.value))
  )
    return fail();
};

export const parseAssistantIncidentReportResponse = (
  value: unknown,
  expectedRunId: string,
): AssistantIncidentReportResponse => {
  if (value === null || typeof value !== 'object') return fail();
  const descriptor = Object.getOwnPropertyDescriptor(value, 'content_state');
  if (descriptor === undefined || !Object.hasOwn(descriptor, 'value')) return fail();
  const state: unknown = descriptor.value;
  const response = closed(
    value,
    state === 'ready'
      ? ['run', 'content_state', 'report', 'citations', 'coverage']
      : ['run', 'content_state'],
  );
  const run = parseRun(response.run);
  if (run.id !== expectedRunId || run.goal !== 'investigate_incident') return fail();
  if (state === 'pending' && (run.state === 'queued' || run.state === 'running'))
    return { run, content_state: state } as AssistantIncidentReportResponse;
  if (state === 'volatile_lost' && run.state === 'completed')
    return { run, content_state: state } as AssistantIncidentReportResponse;
  if (state === 'unavailable') return { run, content_state: state };
  if (state !== 'ready' || run.state !== 'completed') return fail();
  const citations = boundedArray(response.citations, 1, 64).map(parseCitation);
  const references = new Map(citations.map((ref) => [ref.id, ref]));
  if (
    references.size !== citations.length ||
    citations.some(
      (ref) =>
        ref.source_id !== run.scope.source_id ||
        ref.resource_id !== run.scope.resource_id ||
        ref.range.from !== run.scope.range.from ||
        ref.range.to !== run.scope.range.to,
    )
  )
    return fail();
  const coverage = parseCoverage(response.coverage, citations, run.scope.window_minutes);
  const report = closed(response.report, ['timeline', 'hypotheses', 'gaps', 'next_reads']);
  const timeline = boundedArray(report.timeline, citations.length, citations.length).map((value) =>
    observation(value, references, run),
  );
  const facts = new Map(timeline.map((item) => [item.citation_id, item]));
  if (
    facts.size !== citations.length ||
    timeline.some((item, index) => {
      const prior = timeline[index - 1];
      return (
        prior !== undefined &&
        (timestamp(prior.at, true) > timestamp(item.at, true) ||
          (timestamp(prior.at, true) === timestamp(item.at, true) &&
            prior.citation_id >= item.citation_id))
      );
    })
  )
    return fail();
  let previous = -1;
  const hypotheses = boundedArray(report.hypotheses, 0, 4).map((value, index) => {
    const item = closed(value, ['code', 'rank', 'qualified', 'supporting', 'contrary']);
    if (!member(item.code, hypothesisCodes) || item.rank !== index + 1 || item.qualified !== true)
      return fail();
    const order = hypothesisCodes.indexOf(item.code);
    if (order <= previous) return fail();
    previous = order;
    const supporting = codes(item.supporting, [...facts.keys()], 1, 2);
    const contrary = codes(item.contrary, [...facts.keys()], 0, 2);
    const result = {
      code: item.code,
      rank: index + 1,
      qualified: true as const,
      supporting,
      contrary,
    };
    validateHypothesis(result, facts, run);
    return result;
  });
  const gaps = codes(report.gaps, gapCodes, 3, 12);
  if (gapCodes.slice(0, 3).some((code) => !gaps.includes(code))) return fail();
  const next_reads = codes(report.next_reads, nextReadCodes, 5, 5);
  if (next_reads.some((code, index) => code !== nextReadCodes[index])) return fail();
  const parsed = { timeline, hypotheses, gaps, next_reads };
  if (new TextEncoder().encode(JSON.stringify(parsed)).byteLength > 65536) return fail();
  return {
    run,
    content_state: 'ready',
    report: parsed,
    citations,
    coverage,
  } as AssistantIncidentReportResponse;
};
