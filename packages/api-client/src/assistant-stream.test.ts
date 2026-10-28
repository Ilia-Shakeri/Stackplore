import { afterEach, describe, expect, it, vi } from 'vitest';
import { readAssistantStatus } from './assistant-stream';
import { assistantRunId } from './assistant-fixture.test-support';
import type { AssistantStatusMessage } from './assistant-status';

const encode = (text: string) => new TextEncoder().encode(text);
const frame = (sequence = 1) =>
  `event: assistant.status.v1\nid: ${String(sequence)}\ndata: ${JSON.stringify({ run_id: assistantRunId, sequence, version: sequence, state: 'running', at: '2026-10-28T10:15:00.000001Z' })}\n\n`;
const reset = `event: assistant.reset.v1\ndata: ${JSON.stringify({ run_id: assistantRunId, reason: 'replay_gap' })}\n\n`;
const headers = { 'Content-Type': 'text/event-stream; charset=utf-8', 'Cache-Control': 'no-store' };
const response = (text: string) => new Response(text, { headers });
const transport = (reply: Response) => vi.fn<typeof fetch>().mockResolvedValue(reply);
const fault = 'API transport failed with unexpected_response.';

afterEach(() => vi.useRealTimers());

describe('one bounded private status connection', () => {
  it('uses only the exact same-origin protected read and forwards validated metadata', async () => {
    const fetcher = transport(response(frame(3) + frame(4)));
    const messages: AssistantStatusMessage[] = [];
    expect(
      await readAssistantStatus(assistantRunId, {
        fetch: fetcher,
        afterCursor: '2',
        onMessage: (message) => {
          messages.push(message);
        },
      }),
    ).toBe('ended');
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher.mock.calls[0]?.[0]).toBe(`/api/v1/assistant/runs/${assistantRunId}/events`);
    expect(fetcher.mock.calls[0]?.[1]).toMatchObject({
      method: 'GET',
      credentials: 'same-origin',
      mode: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      referrerPolicy: 'no-referrer',
      headers: { Accept: 'text/event-stream', 'Last-Event-ID': '2' },
    });
    expect(messages.map((message) => (message.type === 'status' ? message.id : ''))).toEqual([
      '3',
      '4',
    ]);
    expect(fetcher.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
  });

  it('omits a fresh replay cursor, closes on reset and never reconnects', async () => {
    const cancel = vi.fn();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encode(reset));
      },
      cancel,
    });
    const fetcher = transport(new Response(body, { headers }));
    const onMessage = vi.fn();
    expect(await readAssistantStatus(assistantRunId, { fetch: fetcher, onMessage })).toBe('reset');
    expect(onMessage).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledOnce();
    expect(fetcher).toHaveBeenCalledOnce();
    expect(fetcher.mock.calls[0]?.[1]?.headers).toEqual({ Accept: 'text/event-stream' });
  });

  it('rejects a malformed same-chunk suffix without publishing its valid prefix', async () => {
    const onMessage = vi.fn();
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: transport(response(frame() + 'event: wrong\ndata: private-text\n\n')),
        onMessage,
      }),
    ).rejects.toThrow(fault);
    expect(onMessage).not.toHaveBeenCalled();
  });

  it('finishes a final CR frame and handles a transport chunk above decoder size', async () => {
    const text = ': heartbeat\n\n'.repeat(6000) + frame().replaceAll('\n', '\r');
    const onMessage = vi.fn();
    expect(encode(text).byteLength).toBeGreaterThan(64 * 1024);
    await expect(
      readAssistantStatus(assistantRunId, { fetch: transport(response(text)), onMessage }),
    ).resolves.toBe('ended');
    expect(onMessage).toHaveBeenCalledOnce();
  });

  it.each([
    new Response('private', { status: 401, headers }),
    new Response('private', { headers: { ...headers, 'Content-Type': 'application/json' } }),
    new Response('private', { headers: { 'Content-Type': 'text/event-stream' } }),
    new Response(null, { headers }),
  ])('rejects wrong response metadata and disposes the body %#', async (reply) => {
    const onMessage = vi.fn();
    await expect(
      readAssistantStatus(assistantRunId, { fetch: transport(reply), onMessage }),
    ).rejects.toThrow(fault);
    expect(onMessage).not.toHaveBeenCalled();
    expect(reply.body?.locked ?? false).toBe(false);
  });

  it.each(['1048577', '01', '-1'])('rejects declared response bound %s', async (declared) => {
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: transport(
          new Response('private', { headers: { ...headers, 'Content-Length': declared } }),
        ),
        onMessage: vi.fn(),
      }),
    ).rejects.toMatchObject({ code: 'response_too_large' });
  });

  it('rejects actual byte overflow and leaves no callback or reader lock', async () => {
    const reply = response(': '.repeat(524289));
    const onMessage = vi.fn();
    await expect(
      readAssistantStatus(assistantRunId, { fetch: transport(reply), onMessage }),
    ).rejects.toMatchObject({ code: 'response_too_large' });
    expect(onMessage).not.toHaveBeenCalled();
    expect(reply.body?.locked).toBe(false);
  });

  it('does no network work for an already revoked owner or invalid identity', async () => {
    const owner = new AbortController();
    owner.abort('private-owner-reason');
    const fetcher = transport(response(frame()));
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: fetcher,
        signal: owner.signal,
        onMessage: vi.fn(),
      }),
    ).rejects.toMatchObject({ name: 'AbortError', message: 'The request was aborted.' });
    await expect(
      readAssistantStatus('wrong', { fetch: fetcher, onMessage: vi.fn() }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: fetcher,
        afterCursor: '01',
        onMessage: vi.fn(),
      }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('fences late body callbacks when the owner cancels while reading', async () => {
    const owner = new AbortController();
    let cancelled = false;
    const reply = new Response(
      new ReadableStream<Uint8Array>({
        cancel() {
          cancelled = true;
          return new Promise<void>(() => undefined);
        },
      }),
      { headers },
    );
    const onMessage = vi.fn();
    const pending = readAssistantStatus(assistantRunId, {
      fetch: transport(reply),
      signal: owner.signal,
      onMessage,
    });
    await vi.waitFor(() => {
      expect(reply.body?.locked).toBe(true);
    });
    owner.abort('private-owner-reason');
    await expect(pending).rejects.toMatchObject({
      name: 'AbortError',
      message: 'The request was aborted.',
    });
    expect(cancelled).toBe(true);
    expect(reply.body?.locked).toBe(false);
    expect(onMessage).not.toHaveBeenCalled();
  });

  it('bounds a fetch that ignores cancellation and cancels its later response', async () => {
    vi.useFakeTimers();
    let resolve: ((response: Response) => void) | undefined;
    const fetcher = vi.fn<typeof fetch>().mockImplementation(
      () =>
        new Promise<Response>((done) => {
          resolve = done;
        }),
    );
    const pending = readAssistantStatus(assistantRunId, {
      fetch: fetcher,
      lifetimeMs: 50,
      idleMs: 20,
      onMessage: vi.fn(),
    });
    const checked = expect(pending).rejects.toMatchObject({ code: 'network_error' });
    await vi.advanceTimersByTimeAsync(20);
    await checked;
    const cancel = vi.fn();
    resolve?.(new Response(new ReadableStream<Uint8Array>({ cancel }), { headers }));
    await Promise.resolve();
    expect(cancel).toHaveBeenCalledOnce();
    expect(fetcher).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('keeps a total lifetime despite repeated heartbeats', async () => {
    vi.useFakeTimers();
    const onMessage = vi.fn();
    let controller: ReadableStreamDefaultController<Uint8Array> | undefined;
    const reply = new Response(
      new ReadableStream<Uint8Array>({
        start(value) {
          controller = value;
        },
      }),
      { headers },
    );
    const pending = readAssistantStatus(assistantRunId, {
      fetch: transport(reply),
      lifetimeMs: 50,
      idleMs: 20,
      onMessage,
    });
    const checked = expect(pending).rejects.toMatchObject({ code: 'network_error' });
    for (let index = 0; index < 5; index += 1) {
      await vi.advanceTimersByTimeAsync(9);
      controller?.enqueue(encode(': beat\n\n'));
      await Promise.resolve();
    }
    await vi.advanceTimersByTimeAsync(5);
    await checked;
    expect(onMessage).not.toHaveBeenCalled();
    expect(reply.body?.locked).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('sanitizes fetch and callback faults; never retains an asynchronous callback', async () => {
    const fetcher = vi.fn<typeof fetch>().mockRejectedValue(new Error('private endpoint or text'));
    await expect(
      readAssistantStatus(assistantRunId, { fetch: fetcher, onMessage: vi.fn() }),
    ).rejects.toMatchObject({
      code: 'network_error',
      message: 'API transport failed with network_error.',
    });
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: transport(response(frame())),
        onMessage() {
          throw new Error('private callback');
        },
      }),
    ).rejects.toThrow(fault);
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: transport(response(frame())),
        onMessage: () => Promise.reject(new Error('private asynchronous callback')),
      }),
    ).rejects.toThrow(fault);
  });

  it('fences subsequent same-chunk messages when a callback revokes its owner', async () => {
    const owner = new AbortController();
    const onMessage = vi.fn(() => {
      owner.abort('private-owner-reason');
    });
    await expect(
      readAssistantStatus(assistantRunId, {
        fetch: transport(response(frame() + frame(2))),
        signal: owner.signal,
        onMessage,
      }),
    ).rejects.toMatchObject({ name: 'AbortError', message: 'The request was aborted.' });
    expect(onMessage).toHaveBeenCalledOnce();
  });

  it.each([
    { lifetimeMs: 0 },
    { lifetimeMs: 120001 },
    { idleMs: 15001 },
    { idleMs: 0 },
    { lifetimeMs: 10, idleMs: 11 },
  ])('rejects invalid connection limits before I/O %#', async (limits) => {
    const fetcher = transport(response(frame()));
    await expect(
      readAssistantStatus(assistantRunId, { ...limits, fetch: fetcher, onMessage: vi.fn() }),
    ).rejects.toMatchObject({ code: 'invalid_configuration' });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('reads a replay cursor only once before constructing the request', async () => {
    const fetcher = transport(response(frame(3)));
    const cursor = vi.fn(() => '2');
    await readAssistantStatus(assistantRunId, {
      fetch: fetcher,
      get afterCursor() {
        return cursor();
      },
      onMessage: vi.fn(),
    });
    expect(cursor).toHaveBeenCalledOnce();
    expect(fetcher.mock.calls[0]?.[1]?.headers).toMatchObject({ 'Last-Event-ID': '2' });
  });

  it('caps empty chunks even when a transport starves timer callbacks', async () => {
    const reply = new Response(
      new ReadableStream<Uint8Array>({
        pull(controller) {
          controller.enqueue(new Uint8Array(0));
        },
      }),
      { headers },
    );
    const onMessage = vi.fn();
    await expect(
      readAssistantStatus(assistantRunId, { fetch: transport(reply), onMessage }),
    ).rejects.toMatchObject({ code: 'response_too_large' });
    expect(reply.body?.locked).toBe(false);
    expect(onMessage).not.toHaveBeenCalled();
  });
});
