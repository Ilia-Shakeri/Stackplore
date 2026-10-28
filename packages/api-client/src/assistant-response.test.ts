import { describe, expect, it, vi } from 'vitest';
import publicationFixture from '../../../contracts/fixtures/assistant-ready.json?raw';
import {
  createApiClient,
  createAssistantClient,
  parseAssistantCitation,
  parseAssistantRunReceipt,
  parseAssistantRunResponse,
} from './index';
import {
  assistantCitationFixture,
  assistantCitationId,
  assistantReadyFixture,
  assistantRunFixture,
  assistantRunId,
} from './assistant-fixture.test-support';

const first = <T>(values: readonly T[]): T => {
  const value = values[0];
  if (value === undefined) throw new Error('Fixture field missing');
  return value;
};
const reject = (value: unknown) => {
  expect(() => parseAssistantRunResponse(value, assistantRunId)).toThrow(
    'API transport failed with unexpected_response.',
  );
};

describe('closed private conversation replies', () => {
  it('accepts the same synthetic publication fixture checked by the server boundary', () => {
    const input: unknown = JSON.parse(publicationFixture);
    expect(parseAssistantRunResponse(input, assistantRunId)).toEqual(input);
  });
  it.each(['en', 'fa'])('accepts complete inert %s text and copies the entire result', (locale) => {
    const input = assistantReadyFixture();
    first(input.answer.sections).text =
      locale === 'fa'
        ? 'این منبع در دسترس است؛ داده محدود است.'
        : 'Resource available. Evidence limited.';
    const result = parseAssistantRunResponse(input, assistantRunId);
    expect(result).toEqual(input);
    first(input.answer.sections).text = 'changed';
    input.run.scope.range.from = 'changed';
    first(input.citations).range.to = 'changed';
    expect(result).not.toEqual(input);
  });

  it.each([
    'queued',
    'running',
    'completed',
    'failed',
    'cancelled',
    'expired',
    'interrupted',
    'uncertain',
  ])('keeps %s metadata distinct', (state) => {
    const run = { ...assistantRunFixture(), state };
    expect(parseAssistantRunReceipt({ run }, assistantRunId)).toEqual({ run });
    expect(
      parseAssistantRunResponse({ run, content_state: 'unavailable' }, assistantRunId),
    ).toEqual({ run, content_state: 'unavailable' });
    if (state === 'queued' || state === 'running') {
      expect(
        parseAssistantRunResponse({ run, content_state: 'pending' }, assistantRunId).content_state,
      ).toBe('pending');
    } else reject({ run, content_state: 'pending' });
    if (state === 'completed') {
      expect(
        parseAssistantRunResponse({ run, content_state: 'volatile_lost' }, assistantRunId)
          .content_state,
      ).toBe('volatile_lost');
    } else {
      reject({ run, content_state: 'volatile_lost' });
      reject({ ...assistantReadyFixture(), run });
    }
  });

  it.each([
    '<script>alert(1)</script>',
    '<img src=x>',
    '[go](javascript:alert(1))',
    'https://example.invalid',
    'http://example.invalid',
    '\0',
    '\ud800',
    '\udc00',
    '  ',
    '\u0085',
    'a'.repeat(4097),
    '😀'.repeat(1025),
  ])('rejects unsafe or oversized text case %#', (text) => {
    const input = assistantReadyFixture();
    first(input.answer.sections).text = text;
    reject(input);
  });

  it('accepts exact UTF-8 text limits; rejects total answer bytes above 64 KiB', () => {
    const input = assistantReadyFixture();
    first(input.answer.sections).text = '😀'.repeat(1024);
    expect(parseAssistantRunResponse(input, assistantRunId)).toEqual(input);
    // Escaped controls enlarge JSON independently of decoded text length.
    input.answer.sections = Array.from({ length: 12 }, () => ({
      kind: 'gaps',
      text: '\u0001'.repeat(4096),
      citations: [],
    }));
    reject(input);
  });

  it.each([
    { kind: 'facts', citations: [] },
    { kind: 'facts', citations: [`ev_${'d'.repeat(64)}`] },
    { kind: 'facts', citations: [assistantCitationId, assistantCitationId] },
    { kind: 'gaps', citations: ['https://example.invalid'] },
    { kind: 'instructions', citations: [] },
  ])('rejects ungrounded section %#', (patch) => {
    const input = assistantReadyFixture();
    Object.assign(first(input.answer.sections), patch);
    reject(input);
  });

  it.each(['hypotheses', 'gaps', 'next_checks'])(
    'accepts %s without inventing a fact citation',
    (kind) => {
      const input = assistantReadyFixture();
      input.answer.sections = [{ kind, text: 'Evidence missing.', citations: [] }];
      expect(parseAssistantRunResponse(input, assistantRunId)).toEqual(input);
    },
  );

  it.each([
    { source_id: 'another_source' },
    { resource_id: `r1_${'B'.repeat(42)}A` },
    { range: { from: '2026-10-28T09:00:00Z', to: '2026-10-28T09:15:00Z' } },
    { href: 'https://example.invalid' },
    { observed_at: '2026-02-30T00:00:00Z' },
  ])('rejects wrong locator or extra content %#', (patch) => {
    const input = assistantReadyFixture();
    Object.assign(first(input.citations), patch);
    reject(input);
  });

  it.each([
    { version: 0 },
    { version: 65 },
    { version: 1.5 },
    { version: NaN },
    { id: `ar_${'d'.repeat(64)}` },
    { conversation_id: 'bad' },
    { state: 'done' },
    { goal: 'execute' },
    { usage_uncertain: 1 },
    { usage_conservative: undefined },
    { question: 'private question' },
    { deadline: '2026-10-28T10:15:00Z' },
    { updated_at: '2026-10-28T10:14:59Z' },
    { created_at: '2026-02-30T10:15:00Z' },
    { created_at: '2026-10-28T10:15:00+00:00' },
  ])('rejects invalid durable metadata %#', (patch) => {
    reject({ ...assistantReadyFixture(), run: { ...assistantRunFixture(), ...patch } });
  });

  it('checks microsecond ordering without millisecond truncation', () => {
    const input = assistantReadyFixture();
    const range = { from: '2026-10-28T10:00:00.000002Z', to: '2026-10-28T10:00:00.000001Z' };
    input.run.scope.range = range;
    first(input.citations).range = range;
    reject(input);
    input.run.scope.range = {
      from: '2026-10-28T10:00:00.0000001Z',
      to: '2026-10-28T10:15:00.0000001Z',
    };
    reject(input);
  });

  it('rejects missing, unknown and executable fields at every nesting level', () => {
    const targets = [
      (input: ReturnType<typeof assistantReadyFixture>) => input,
      (input: ReturnType<typeof assistantReadyFixture>) => input.run,
      (input: ReturnType<typeof assistantReadyFixture>) => input.run.scope,
      (input: ReturnType<typeof assistantReadyFixture>) => input.run.scope.range,
      (input: ReturnType<typeof assistantReadyFixture>) => input.answer,
      (input: ReturnType<typeof assistantReadyFixture>) => first(input.answer.sections),
      (input: ReturnType<typeof assistantReadyFixture>) => first(input.citations),
    ];
    for (const target of targets) {
      for (const mutation of ['extra', 'missing', 'getter', 'symbol', 'prototype']) {
        const input = assistantReadyFixture();
        const object = target(input);
        const field = first(Object.keys(object));
        const getter = vi.fn(() => {
          throw new Error('private getter payload');
        });
        if (mutation === 'extra')
          Object.defineProperty(object, 'private_text', { value: 'private content' });
        if (mutation === 'missing') Reflect.deleteProperty(object, field);
        if (mutation === 'getter') Object.defineProperty(object, field, { get: getter });
        if (mutation === 'symbol')
          Object.defineProperty(object, Symbol('hidden'), { value: 'private' });
        if (mutation === 'prototype') Object.setPrototypeOf(object, { hidden: 'private' });
        reject(input);
        expect(getter).not.toHaveBeenCalled();
      }
    }
  });

  it('rejects malformed arrays and content on metadata-only replies', () => {
    for (const sections of [
      [],
      Array.from({ length: 13 }, () => assistantReadyFixture().answer.sections[0]),
      new Array(1),
    ]) {
      reject({ ...assistantReadyFixture(), answer: { sections } });
    }
    for (const citations of [[], [assistantCitationFixture(), assistantCitationFixture()]])
      reject({ ...assistantReadyFixture(), citations });
    reject({
      run: assistantRunFixture(),
      content_state: 'volatile_lost',
      answer: { sections: [] },
    });
    reject(null);
    reject([]);
    reject({ run: assistantRunFixture(), content_state: 'unknown' });
  });

  it('binds receipt and citation replies to requested identifiers', () => {
    expect(parseAssistantCitation(assistantCitationFixture(), assistantCitationId)).toEqual(
      assistantCitationFixture(),
    );
    expect(() =>
      parseAssistantCitation(assistantCitationFixture(), `ev_${'d'.repeat(64)}`),
    ).toThrow('unexpected_response');
    expect(() =>
      parseAssistantRunReceipt({ run: assistantRunFixture() }, `ar_${'d'.repeat(64)}`),
    ).toThrow('unexpected_response');
  });

  it.each([
    { source_id: 'another_source' },
    { resource_id: `r1_${'B'.repeat(42)}A` },
    {
      window_minutes: 5,
      range: { from: '2026-10-28T10:10:00.000001Z', to: '2026-10-28T10:15:00.000001Z' },
    },
  ])('admission cannot silently change the selected scope %#', async (patch) => {
    const run = assistantRunFixture();
    Object.assign(run.scope, patch);
    const fetchMock = vi.fn<typeof fetch>().mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ run }), {
          status: 202,
          headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': 'req_assistant_scope_01',
          },
        }),
      ),
    );
    const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
    await expect(
      client.admit(
        {
          source_id: 'src_demo_01',
          resource_id: `r1_${'A'.repeat(43)}`,
          window_minutes: 15,
          locale: 'en',
          question: 'Explain selected resource.',
        },
        { csrfToken: 'A'.repeat(43), idempotencyKey: 'c'.repeat(64) },
      ),
    ).rejects.toMatchObject({ code: 'unexpected_response' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('admission cannot substitute an explicit conversation', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ run: assistantRunFixture() }), {
          status: 202,
          headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': 'req_assistant_scope_02',
          },
        }),
      ),
    );
    const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
    await expect(
      client.admit(
        {
          source_id: 'src_demo_01',
          resource_id: `r1_${'A'.repeat(43)}`,
          window_minutes: 15,
          locale: 'en',
          question: 'Explain selected resource.',
          conversation_id: `ac_${'d'.repeat(64)}`,
        },
        { csrfToken: 'A'.repeat(43), idempotencyKey: 'c'.repeat(64) },
      ),
    ).rejects.toMatchObject({ code: 'unexpected_response' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('all client methods reject malformed JSON replies without resending or reflecting content', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation(() =>
      Promise.resolve(
        new Response('{"private":"do not expose"}', {
          headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': 'req_assistant_reply_01',
          },
        }),
      ),
    );
    const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
    const calls = [
      () => client.get(assistantRunId),
      () => client.cancel(assistantRunId, { csrfToken: 'A'.repeat(43) }),
      () => client.citation(assistantRunId, assistantCitationId),
      () =>
        client.admit(
          {
            source_id: 'src_demo_01',
            resource_id: `r1_${'A'.repeat(43)}`,
            window_minutes: 15,
            locale: 'fa',
            question: 'بررسی منبع',
          },
          { csrfToken: 'A'.repeat(43), idempotencyKey: 'c'.repeat(64) },
        ),
    ];
    for (const call of calls)
      await expect(call()).rejects.toMatchObject({
        code: 'unexpected_response',
        message: 'API transport failed with unexpected_response.',
      });
    expect(fetchMock).toHaveBeenCalledTimes(4);
  });

  it.each(['admit', 'get', 'cancel', 'citation'])(
    'rejects an unexpected successful HTTP status for %s',
    async (method) => {
      const data =
        method === 'get'
          ? assistantReadyFixture()
          : method === 'citation'
            ? assistantCitationFixture()
            : { run: assistantRunFixture() };
      const fetchMock = vi.fn<typeof fetch>().mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify(data), {
            status: method === 'admit' ? 200 : 201,
            headers: {
              'Content-Type': 'application/json',
              'X-Request-ID': 'req_assistant_status_01',
            },
          }),
        ),
      );
      const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
      const result =
        method === 'get'
          ? client.get(assistantRunId)
          : method === 'cancel'
            ? client.cancel(assistantRunId, { csrfToken: 'A'.repeat(43) })
            : method === 'citation'
              ? client.citation(assistantRunId, assistantCitationId)
              : client.admit(
                  {
                    source_id: 'src_demo_01',
                    resource_id: `r1_${'A'.repeat(43)}`,
                    window_minutes: 15,
                    locale: 'en',
                    question: 'Explain selected resource.',
                  },
                  { csrfToken: 'A'.repeat(43), idempotencyKey: 'c'.repeat(64) },
                );
      await expect(result).rejects.toMatchObject({ code: 'unexpected_response' });
      expect(fetchMock).toHaveBeenCalledTimes(1);
    },
  );

  it('checks cancellation again before releasing a resolved reply', async () => {
    const controller = new AbortController();
    const base = createApiClient();
    vi.spyOn(base, 'request').mockImplementation(() => {
      controller.abort(new Error('private cancellation reason'));
      return Promise.resolve({
        data: assistantReadyFixture(),
        status: 200,
        requestId: 'req_assistant_abort_01',
      });
    });
    const client = createAssistantClient(base);
    await expect(client.get(assistantRunId, { signal: controller.signal })).rejects.toMatchObject({
      name: 'AbortError',
      message: 'The request was aborted.',
    });
  });

  it('preserves safe transport metadata on a completely validated ready reply', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify(assistantReadyFixture()), {
          headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': 'req_assistant_ready_01',
          },
        }),
      ),
    );
    const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
    await expect(client.get(assistantRunId)).resolves.toEqual({
      data: assistantReadyFixture(),
      status: 200,
      requestId: 'req_assistant_ready_01',
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
