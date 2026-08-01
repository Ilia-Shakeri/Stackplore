import { describe, expectTypeOf, it } from 'vitest';

import type {
  Alert,
  AuditActor,
  CapabilitySet,
  DataAllowedCurrentMetricSet,
  DataAllowedUnknownHealthSnapshot,
  HealthEvidence,
  HealthResult,
  KnownHealthSnapshot,
  MetricSample,
  MetricSampleSet,
  NoDataCurrentMetricResult,
  NoDataCurrentMetricSet,
  NoDataEnvironmentPage,
  NoDataEventPage,
  NoDataHealthResult,
  NoDataLogBatch,
  NoDataRelationshipPage,
  NoDataResourcePage,
  NoDataUnknownHealthSnapshot,
  ObservationStatus,
  PageInfo,
  Relationship,
  RelationshipEvidence,
  Scope,
} from './index';

describe('API contract types', () => {
  it('keeps availability, scope, and page states truthful', () => {
    expectTypeOf<{
      completeness: 'partial';
      observed_at: string;
      reason_code: string;
      state: 'available';
    }>().toExtend<ObservationStatus>();
    expectTypeOf<{
      completeness: 'partial';
      observed_at: string;
      reason_code: string;
      state: 'unavailable';
    }>().not.toExtend<ObservationStatus>();
    expectTypeOf<{ kind: 'global'; name: '' }>().toExtend<Scope>();
    expectTypeOf<{ kind: 'global'; name: 'project-a' }>().not.toExtend<Scope>();
    expectTypeOf<{
      has_more: true;
      limit: number;
      next_cursor: string;
    }>().toExtend<PageInfo>();
    expectTypeOf<{ has_more: true; limit: number }>().not.toExtend<PageInfo>();
  });

  it('keeps health evidence non-empty and metric units paired', () => {
    expectTypeOf<KnownHealthSnapshot['evidence']>().toExtend<
      [HealthEvidence, ...HealthEvidence[]]
    >();
    expectTypeOf<[]>().not.toExtend<KnownHealthSnapshot['evidence']>();
    expectTypeOf<DataAllowedUnknownHealthSnapshot['evidence']>().toEqualTypeOf<HealthEvidence[]>();
    expectTypeOf<NoDataUnknownHealthSnapshot['evidence']>().toEqualTypeOf<[]>();
    expectTypeOf<Relationship['evidence']>().toExtend<
      [RelationshipEvidence, ...RelationshipEvidence[]]
    >();
    expectTypeOf<[]>().not.toExtend<Relationship['evidence']>();
    expectTypeOf<{
      id: string;
      name: 'cpu.usage.cores';
      resource_id: string;
      sampled_at: string;
      synthetic: false;
      unit: 'cores';
      value: number;
    }>().toExtend<MetricSample>();
    expectTypeOf<{
      id: string;
      name: 'cpu.usage.cores';
      resource_id: string;
      sampled_at: string;
      synthetic: false;
      unit: 'bytes';
      value: number;
    }>().not.toExtend<MetricSample>();
    expectTypeOf<NonNullable<MetricSampleSet['restart.count']>['unit']>().toEqualTypeOf<'count'>();
  });

  it('keeps no-data results empty and branch correlation exact', () => {
    expectTypeOf<NoDataHealthResult['items']>().toEqualTypeOf<[]>();
    expectTypeOf<NoDataResourcePage['items']>().toEqualTypeOf<[]>();
    expectTypeOf<NoDataEnvironmentPage['items']>().toEqualTypeOf<[]>();
    expectTypeOf<NoDataRelationshipPage['items']>().toEqualTypeOf<[]>();
    expectTypeOf<NoDataLogBatch['items']>().toEqualTypeOf<[]>();
    expectTypeOf<NoDataCurrentMetricResult['items']>().toEqualTypeOf<[]>();
    expectTypeOf<NoDataEventPage['items']>().toEqualTypeOf<[]>();
    expectTypeOf<Record<PropertyKey, never>>().toExtend<NoDataCurrentMetricSet['samples']>();
    expectTypeOf<{ sample: MetricSample }>().not.toExtend<NoDataCurrentMetricSet['samples']>();
    expectTypeOf<DataAllowedCurrentMetricSet['samples']>().toEqualTypeOf<MetricSampleSet>();
    expectTypeOf<NoDataHealthResult>().toExtend<HealthResult>();
  });

  it('keeps lifecycle fields and capability positions exact', () => {
    type ActiveAlert = Extract<Alert, { state: 'pending' | 'firing' }>;
    type SystemActor = Extract<AuditActor, { kind: 'system' }>;
    type UserActor = Extract<AuditActor, { kind: 'user' }>;

    expectTypeOf<'resolved_at'>().not.toExtend<keyof ActiveAlert>();
    expectTypeOf<'id'>().not.toExtend<keyof SystemActor>();
    expectTypeOf<'id'>().toExtend<keyof UserActor>();
    expectTypeOf<CapabilitySet['items'][0]['name']>().toEqualTypeOf<'inventory.list'>();
    expectTypeOf<CapabilitySet['items'][8]['name']>().toEqualTypeOf<'event.watch'>();
  });
});
