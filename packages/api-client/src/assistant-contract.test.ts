import { describe, expectTypeOf, it } from 'vitest';

import type {
  AssistantAnswer,
  AssistantAnswerSection,
  AssistantCitationId,
  AssistantFactSection,
  AssistantLocale,
  AssistantLostResponse,
  AssistantOtherSection,
  AssistantPendingResponse,
  AssistantReadyResponse,
  AssistantResetEvent,
  AssistantResourceCitation,
  AssistantRun,
  AssistantRunRequest,
  AssistantRunResponse,
  AssistantRunState,
  AssistantStatusEvent,
  AssistantUnavailableResponse,
  AssistantWindowMinutes,
} from './index';

describe('private assistant contract types', () => {
  it('accepts only the selected local resource journey', () => {
    expectTypeOf<keyof AssistantRunRequest>().toEqualTypeOf<
      'source_id' | 'resource_id' | 'window_minutes' | 'locale' | 'question' | 'conversation_id'
    >();
    expectTypeOf<AssistantLocale>().toEqualTypeOf<'en' | 'fa'>();
    expectTypeOf<AssistantWindowMinutes>().toEqualTypeOf<5 | 15 | 60 | 360 | 1440>();
    expectTypeOf<AssistantRun['goal']>().toEqualTypeOf<'explain_resource'>();
    expectTypeOf<
      Extract<keyof AssistantRun, 'question' | 'profile_id' | 'lease_owner' | 'session_digest'>
    >().toEqualTypeOf<never>();
    expectTypeOf<keyof AssistantRun['scope']>().toEqualTypeOf<
      'source_id' | 'resource_id' | 'window_minutes' | 'range'
    >();
  });

  it('keeps ready, pending, lost and withheld content truthful', () => {
    expectTypeOf<AssistantRunState>().toEqualTypeOf<
      | 'queued'
      | 'running'
      | 'completed'
      | 'failed'
      | 'cancelled'
      | 'expired'
      | 'interrupted'
      | 'uncertain'
    >();
    expectTypeOf<AssistantReadyResponse['run']['state']>().toEqualTypeOf<'completed'>();
    expectTypeOf<AssistantPendingResponse['run']['state']>().toEqualTypeOf<'queued' | 'running'>();
    expectTypeOf<AssistantLostResponse['run']['state']>().toEqualTypeOf<'completed'>();
    expectTypeOf<AssistantUnavailableResponse['run']['state']>().toEqualTypeOf<AssistantRunState>();
    expectTypeOf<
      Extract<AssistantRunResponse, { content_state: 'ready' }>
    >().toEqualTypeOf<AssistantReadyResponse>();
    expectTypeOf<AssistantPendingResponse['content_state']>().toEqualTypeOf<'pending'>();
    expectTypeOf<AssistantLostResponse['content_state']>().toEqualTypeOf<'volatile_lost'>();
    expectTypeOf<AssistantUnavailableResponse['content_state']>().toEqualTypeOf<'unavailable'>();
    expectTypeOf<keyof AssistantPendingResponse>().toEqualTypeOf<'run' | 'content_state'>();
    expectTypeOf<keyof AssistantLostResponse>().toEqualTypeOf<'run' | 'content_state'>();
    expectTypeOf<keyof AssistantUnavailableResponse>().toEqualTypeOf<'run' | 'content_state'>();
  });

  it('requires a complete answer and cited facts without inventing four required sections', () => {
    expectTypeOf<AssistantAnswerSection['kind']>().toEqualTypeOf<
      'facts' | 'hypotheses' | 'gaps' | 'next_checks'
    >();
    expectTypeOf<AssistantAnswer['sections']>().toExtend<
      [AssistantAnswerSection, ...AssistantAnswerSection[]]
    >();
    expectTypeOf<[]>().not.toExtend<AssistantAnswer['sections']>();
    expectTypeOf<[]>().not.toExtend<AssistantFactSection['citations']>();
    expectTypeOf<AssistantFactSection['citations']>().toExtend<
      [AssistantCitationId, ...AssistantCitationId[]]
    >();
    expectTypeOf<[]>().toExtend<AssistantOtherSection['citations']>();
    expectTypeOf<AssistantReadyResponse['citations']>().toEqualTypeOf<
      [AssistantResourceCitation]
    >();
    expectTypeOf<AssistantResourceCitation['kind']>().toEqualTypeOf<'resource'>();
    expectTypeOf<keyof AssistantResourceCitation>().toEqualTypeOf<
      'id' | 'kind' | 'source_id' | 'resource_id' | 'observed_at' | 'range'
    >();
  });

  it('keeps streamed events limited to private status and reset control', () => {
    expectTypeOf<keyof AssistantStatusEvent>().toEqualTypeOf<
      'run_id' | 'sequence' | 'version' | 'state' | 'at'
    >();
    expectTypeOf<keyof AssistantResetEvent>().toEqualTypeOf<'run_id' | 'reason'>();
    expectTypeOf<AssistantResetEvent['reason']>().toEqualTypeOf<
      'replay_gap' | 'cursor_ahead' | 'replay_expired' | 'queue_overflow'
    >();
    expectTypeOf<AssistantStatusEvent['state']>().toEqualTypeOf<AssistantRunState>();
  });
});
