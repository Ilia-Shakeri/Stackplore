import { describe, expect, it } from 'vitest';

import { draftInput, emptyDraft, recordDraft } from './alert-rule-draft';

const resourceId = `r1_${'A'.repeat(43)}`;
const validDraft = () => ({
  ...emptyDraft(),
  displayName: 'Gateway health',
  sourceId: 'source_demo',
  resourceId,
});

describe('alert rule drafts', () => {
  it('preserves an optional event resource while editing a persisted rule', () => {
    const timestamp = '2026-08-16T08:00:00Z';
    const draft = recordDraft({
      id: 'rule_event',
      source_id: 'source_demo',
      resource_id: resourceId,
      environment_id: 'env_demo',
      display_name: 'Event severity',
      kind: 'event_severity_match',
      enabled: true,
      severity: 'critical',
      minimum_event_severity: 'warning',
      evaluation_interval_seconds: 30,
      pending_for_seconds: 0,
      created_at: timestamp,
      updated_at: timestamp,
      version: 1,
    });
    expect(draftInput(draft)).toEqual({
      source_id: 'source_demo',
      environment_id: 'env_demo',
      resource_id: resourceId,
      display_name: 'Event severity',
      kind: 'event_severity_match',
      enabled: true,
      severity: 'critical',
      minimum_event_severity: 'warning',
      evaluation_interval_seconds: 30,
      pending_for_seconds: 0,
    });
  });

  it('keeps provider connection rules source-scoped after changing the predicate', () => {
    expect(
      draftInput({ ...validDraft(), kind: 'provider_connection_lost', environmentId: 'env_demo' }),
    ).toEqual({
      display_name: 'Gateway health',
      source_id: 'source_demo',
      enabled: true,
      kind: 'provider_connection_lost',
      severity: 'warning',
      evaluation_interval_seconds: 30,
      pending_for_seconds: 60,
    });
  });

  it.each([
    { pendingSeconds: '' },
    { evaluationSeconds: '0' },
    { resourceId: 'bad' },
    { sourceId: 'bad source' },
    { environmentId: 'bad environment' },
    { kind: 'cpu_threshold' as const, thresholdValue: '' },
    { kind: 'restart_threshold' as const, thresholdCount: '1.5' },
    { kind: 'memory_threshold' as const, windowSeconds: '86401' },
  ])('rejects incomplete and out-of-range drafts %j', (change) => {
    expect(draftInput({ ...validDraft(), ...change })).toBeNull();
  });
});
