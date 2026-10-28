import { describe, expect, it } from 'vitest';
import {
  createAssistantStatusDecoder,
  parseAssistantResetEvent,
  parseAssistantStatusEvent,
  type AssistantStatusMessage,
} from './index';
import { assistantRunId } from './assistant-fixture.test-support';

const encode = (text: string) => new TextEncoder().encode(text);
const status = (sequence = 1, version = sequence) => ({
  run_id: assistantRunId,
  sequence,
  version,
  state: 'running',
  at: '2026-10-28T10:15:00.000001Z',
});
const frame = (sequence = 1, data: unknown = status(sequence)) =>
  `event: assistant.status.v1\nid: ${String(sequence)}\ndata: ${JSON.stringify(data)}\n\n`;
const resetFrame = (reason = 'replay_gap') =>
  `event: assistant.reset.v1\ndata: ${JSON.stringify({ run_id: assistantRunId, reason })}\n\n`;
const fault = 'API transport failed with unexpected_response.';

describe('private status-only byte decoding', () => {
  it.each(['\n', '\r\n', '\r'])('handles byte boundaries with line ending %#', (ending) => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    const bytes = encode(`\uFEFF: ضربان\n\n${frame(3)}${frame(4)}`.replaceAll('\n', ending));
    const messages: AssistantStatusMessage[] = [];
    for (const byte of bytes) messages.push(...decoder.push(new Uint8Array([byte])));
    messages.push(...decoder.finish());
    expect(messages).toEqual(
      [3, 4].map((sequence) => ({ type: 'status', id: String(sequence), data: status(sequence) })),
    );
    expect(() => decoder.push(encode(frame(5)))).toThrow(fault);
  });

  it('handles every possible two-chunk split of a valid replay', () => {
    const bytes = encode(frame(3) + frame(4));
    for (let split = 0; split <= bytes.length; split += 1) {
      const decoder = createAssistantStatusDecoder(assistantRunId, '2');
      const messages = [
        ...decoder.push(bytes.subarray(0, split)),
        ...decoder.push(bytes.subarray(split)),
        ...decoder.finish(),
      ];
      expect(messages.map((message) => (message.type === 'status' ? message.id : ''))).toEqual([
        '3',
        '4',
      ]);
    }
  });

  it('accepts one snapshot jump from zero, then requires contiguous status IDs', () => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    expect(decoder.push(encode(frame(50)))).toHaveLength(1);
    expect(() => decoder.push(encode(frame(52)))).toThrow(fault);
    expect(() => decoder.finish()).toThrow(fault);
  });

  it.each(['replay_gap', 'cursor_ahead', 'replay_expired', 'queue_overflow'])(
    'requires caller recovery after %s without a new dispatch',
    (reason) => {
      const decoder = createAssistantStatusDecoder(assistantRunId, '9223372036854775807');
      expect(decoder.push(encode(resetFrame(reason)))).toEqual([
        { type: 'reset', data: { run_id: assistantRunId, reason } },
      ]);
      expect(decoder.push(encode(': heartbeat\n\n'))).toEqual([]);
      expect(() => decoder.push(encode(frame()))).toThrow(fault);
    },
  );

  it('accepts a reset with an optional canonical ID without advancing any cursor', () => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    expect(decoder.push(encode('id: 9223372036854775807\n' + resetFrame()))).toHaveLength(1);
    expect(decoder.finish()).toEqual([]);
    const invalid = createAssistantStatusDecoder(assistantRunId);
    expect(() => invalid.push(encode('id: 01\n' + resetFrame()))).toThrow(fault);
  });

  it.each(['-1', '+1', '01', ' 1', '1 ', '1\n', '1.0', '', '9223372036854775808', '9'.repeat(100)])(
    'rejects invalid caller cursor %#',
    (after) => {
      expect(() => createAssistantStatusDecoder(assistantRunId, after)).toThrow(
        'API transport failed with invalid_request.',
      );
    },
  );

  it.each([
    { run_id: `ar_${'d'.repeat(64)}` },
    { run_id: assistantRunId + '\n' },
    { sequence: 0 },
    { sequence: 65 },
    { sequence: 1.5 },
    { sequence: '1' },
    { version: 0 },
    { version: 65 },
    { version: 1.5 },
    { state: 'unknown' },
    { at: '2026-02-30T00:00:00Z' },
    { question: 'private input' },
    { answer: 'private output' },
    { citations: [] },
  ])('rejects invalid or content-bearing status %#', (patch) => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    expect(() => decoder.push(encode(frame(1, { ...status(), ...patch })))).toThrow(fault);
  });

  it.each([
    'cancelled',
    'completed',
    'expired',
    'failed',
    'interrupted',
    'uncertain',
    'queued',
    'running',
  ])('preserves the %s status state', (state) => {
    expect(parseAssistantStatusEvent({ ...status(), state }, assistantRunId).state).toBe(state);
  });

  it('enforces replay order, ID equality and nondecreasing versions', () => {
    for (const suffix of [frame(1), frame(3), frame(2, status(3)), frame(2, status(2, 1))]) {
      const decoder = createAssistantStatusDecoder(assistantRunId);
      decoder.push(encode(frame(1, status(1, 5))));
      expect(() => decoder.push(encode(suffix))).toThrow(fault);
    }
    const future = createAssistantStatusDecoder(assistantRunId, '65');
    expect(() => future.push(encode(frame(64)))).toThrow(fault);
  });

  it.each([
    'data: {}\n\n',
    'event: assistant.status.v1\n\n',
    frame().replace('event: assistant.status.v1', 'event: completion.chunk'),
    'retry: 1\n' + frame(),
    'unknown: private payload\n' + frame(),
    'data: {}\n' + frame(),
    'id: 1\n' + frame(),
    'event: assistant.status.v1\n' + frame(),
    frame().replace('id: 1', 'id: 01'),
    frame().replace('id: 1', 'id: \0'),
    frame().replace('data: {', 'data: private{'),
    frame().replace('"sequence":1', '"sequence":1,"sequence":1'),
    frame().replace('"sequence":1', '"sequence":1,"\\u0073equence":1'),
    resetFrame().replace('"reason":"replay_gap"', '"reason":"replay_gap","reason":"replay_gap"'),
    frame().replace('data:', 'data:\n'),
  ])('rejects ambiguous or unexpected framing %#', (text) => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    expect(() => decoder.push(encode(text))).toThrow(fault);
  });

  it('never releases a partial successful prefix if the same chunk contains a bad frame', () => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    expect(() =>
      decoder.push(encode(frame() + frame(2, { ...status(2), answer: 'private payload' }))),
    ).toThrow(fault);
    expect(() => decoder.push(encode(frame(2)))).toThrow(fault);
  });

  it('bounds full raw frame bytes including ignored comments, and bounds caller chunks', () => {
    const data = frame();
    const exact = ':' + 'x'.repeat(16384 - encode(data).byteLength - 2) + '\n' + data;
    expect(encode(exact).byteLength).toBe(16384);
    const good = createAssistantStatusDecoder(assistantRunId);
    expect(good.push(encode(exact))).toHaveLength(1);
    expect(good.finish()).toEqual([]);
    const largeFrame = createAssistantStatusDecoder(assistantRunId);
    expect(() => largeFrame.push(encode(':' + 'x'.repeat(16384)))).toThrow(fault);
    const largeChunk = createAssistantStatusDecoder(assistantRunId);
    expect(() => largeChunk.push(new Uint8Array(65537))).toThrow(fault);
  });

  it('accepts all 64 statuses and never accepts another sequence', () => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    const messages = decoder.push(
      encode(Array.from({ length: 64 }, (_, index) => frame(index + 1)).join('')),
    );
    expect(messages).toHaveLength(64);
    expect(() => decoder.push(encode(frame(65)))).toThrow(fault);
  });

  it('rejects truncated frames and malformed UTF-8 without reflecting content', () => {
    for (const text of [
      frame().slice(0, -1),
      'event: assistant.status.v1\n',
      'data: {"private":"hidden"',
    ]) {
      const decoder = createAssistantStatusDecoder(assistantRunId);
      decoder.push(encode(text));
      expect(() => decoder.finish()).toThrow(fault);
    }
    for (const bytes of [
      new Uint8Array([58, 32, 0xc0, 0x80, 10, 10]),
      new Uint8Array([58, 32, 0xff, 10, 10]),
    ]) {
      const decoder = createAssistantStatusDecoder(assistantRunId);
      expect(() => decoder.push(bytes)).toThrow(fault);
    }
  });

  it('rejects missing fields, accessors and unknown reset reasons without executing content', () => {
    expect(() =>
      parseAssistantResetEvent({ run_id: assistantRunId, reason: 'new_request' }, assistantRunId),
    ).toThrow(fault);
    expect(() =>
      parseAssistantResetEvent(
        { run_id: assistantRunId, reason: 'replay_gap', answer: 'hidden' },
        assistantRunId,
      ),
    ).toThrow(fault);
    const input = { ...status() };
    Object.defineProperty(input, 'at', {
      get() {
        throw new Error('private getter');
      },
    });
    expect(() => parseAssistantStatusEvent(input, assistantRunId)).toThrow(fault);
    expect(() => parseAssistantStatusEvent({}, assistantRunId)).toThrow(fault);
  });

  it('closes idempotently on cancellation and rejects all late frames', () => {
    const decoder = createAssistantStatusDecoder(assistantRunId);
    decoder.push(encode('data: {"question":"private partial input'));
    decoder.close();
    decoder.close();
    expect(() => decoder.push(encode(frame()))).toThrow(fault);
    expect(() => decoder.finish()).toThrow(fault);
    decoder.close();
  });

  it.each(['../run', assistantRunId + '\n', assistantRunId.toUpperCase(), 'x'.repeat(1024)])(
    'rejects caller run identity %#',
    (run) => {
      expect(() => createAssistantStatusDecoder(run)).toThrow(
        'API transport failed with invalid_request.',
      );
    },
  );
});
