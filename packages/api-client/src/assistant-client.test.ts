import { describe, expect, it, vi } from 'vitest';
import {
  ApiTransportError,
  createApiClient,
  createAssistantClient,
  createAssistantIdempotencyKey,
  type AssistantRunRequest,
} from './index';

const run = `ar_${'a'.repeat(64)}`;
const citation = `ev_${'b'.repeat(64)}`;
const input: AssistantRunRequest = {
  source_id: 'src_demo_01',
  resource_id: `r1_${'A'.repeat(43)}`,
  window_minutes: 15,
  locale: 'fa',
  question: 'چرا این منبع در دسترس نیست؟',
};
const options = { csrfToken: 'A'.repeat(43), idempotencyKey: 'c'.repeat(64) };
const response = () =>
  new Response('{}', {
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_assistant_test_01' },
  });
const fixture = () => {
  const fetchMock = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response()));
  const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
  return { client, fetchMock };
};

describe('explicit private run requests', () => {
  it('constructs only the selected local body and protected same-origin request', async () => {
    const { client, fetchMock } = fixture();
    expect(fetchMock).not.toHaveBeenCalled();
    await client.admit(input, options);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe('/api/v1/assistant/runs');
    expect(init).toMatchObject({
      method: 'POST',
      cache: 'no-store',
      credentials: 'same-origin',
      mode: 'same-origin',
      redirect: 'error',
      referrerPolicy: 'no-referrer',
    });
    if (typeof init?.body !== 'string' || typeof url !== 'string')
      throw new Error('Expected bounded JSON request');
    expect(JSON.parse(init.body)).toEqual(input);
    const headers = new Headers(init.headers);
    expect(headers.get('Idempotency-Key')).toBe(options.idempotencyKey);
    expect(headers.get('X-Stackplore-CSRF')).toBe(options.csrfToken);
    expect(url).not.toContain(options.idempotencyKey);
  });

  it('keeps reads and citations bodyless and cannot accept route overrides in options', async () => {
    const { client, fetchMock } = fixture();
    const controller = new AbortController();
    const extra = {
      signal: controller.signal,
      path: '/operator/unsafe',
      method: 'DELETE',
      body: { question: 'hidden' },
      headers: { Authorization: 'hidden' },
    };
    await client.get(run, extra);
    await client.citation(run, citation, extra);
    for (const [index, path] of [
      `/api/v1/assistant/runs/${run}`,
      `/api/v1/assistant/runs/${run}/citations/${citation}`,
    ].entries()) {
      const [url, init] = fetchMock.mock.calls[index] ?? [];
      expect(url).toBe(path);
      expect(init?.method).toBe('GET');
      expect(init?.body).toBeUndefined();
      expect(init?.signal).toBe(controller.signal);
      expect(new Headers(init?.headers).has('Authorization')).toBe(false);
    }
    await client.cancel(run, { ...options, ...extra });
    const [url, init] = fetchMock.mock.calls[2] ?? [];
    expect(url).toBe(`/api/v1/assistant/runs/${run}/cancel`);
    expect(init?.method).toBe('POST');
    expect(init?.body).toBeUndefined();
    expect(new Headers(init?.headers).get('X-Stackplore-CSRF')).toBe(options.csrfToken);
  });

  it('never resends a failed dispatch; an explicit retry retains its exact key and input', async () => {
    const { client, fetchMock } = fixture();
    fetchMock.mockRejectedValueOnce(new Error('private upstream text'));
    await expect(client.admit(input, options)).rejects.toMatchObject({ code: 'network_error' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    await client.admit(input, options);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const first = fetchMock.mock.calls[0]?.[1],
      second = fetchMock.mock.calls[1]?.[1];
    expect(first?.body).toBe(second?.body);
    expect(new Headers(first?.headers).get('Idempotency-Key')).toBe(
      new Headers(second?.headers).get('Idempotency-Key'),
    );
  });

  it('rejects extra fields, accessors, symbols and non-plain input before network I/O', () => {
    const { client, fetchMock } = fixture();
    const getter = vi.fn(() => input.question);
    const accessor = { ...input };
    Object.defineProperty(accessor, 'question', { get: getter });
    const symbol = { ...input, [Symbol('extra')]: 'hidden' };
    const inherited = Object.assign(
      Object.create({ privateEndpoint: 'https://example.invalid' }) as object,
      input,
    );
    const cases = [
      { ...input, endpoint: 'https://example.invalid' },
      { ...input, conversation_id: undefined },
      accessor,
      symbol,
      inherited,
    ];
    for (const value of cases)
      expect(() => client.admit(value as AssistantRunRequest, options)).toThrow(ApiTransportError);
    expect(getter).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('enforces exact UTF-8 limits without silently repairing malformed Unicode', async () => {
    const { client, fetchMock } = fixture();
    await client.admit({ ...input, question: '😀'.repeat(2048) }, options);
    for (const question of [
      '😀'.repeat(2049),
      'a'.repeat(8193),
      '\ud800',
      '\udc00',
      ' ',
      '\u0000',
    ]) {
      expect(() => client.admit({ ...input, question }, options)).toThrow(ApiTransportError);
    }
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('rejects malformed scope and private path/key values without reflecting them', () => {
    const { client, fetchMock } = fixture();
    const patches: Record<string, unknown>[] = [
      { source_id: '../private' },
      { resource_id: 'r1_bad' },
      { window_minutes: 16 },
      { locale: 'de' },
      { conversation_id: 'caller-name' },
    ];
    for (const patch of patches)
      expect(() => client.admit({ ...input, ...patch }, options)).toThrow(ApiTransportError);
    for (const bad of ['../private', 'x'.repeat(1024), run + '\n', run + '\r']) {
      expect(() => client.get(bad)).toThrow(ApiTransportError);
      expect(() => client.cancel(bad, options)).toThrow(ApiTransportError);
      expect(() => client.citation(bad, citation)).toThrow(ApiTransportError);
    }
    expect(() => client.citation(run, 'https://example.invalid')).toThrow(ApiTransportError);
    for (const bad of [
      'caller-key',
      options.idempotencyKey + '\n',
      options.idempotencyKey.toUpperCase(),
    ]) {
      expect(() => client.admit(input, { ...options, idempotencyKey: bad })).toThrow(
        'API transport failed with invalid_request.',
      );
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('passes cancellation without storing or sending an already-aborted question', async () => {
    const { client, fetchMock } = fixture();
    const controller = new AbortController();
    controller.abort();
    await expect(
      client.admit(input, { ...options, signal: controller.signal }),
    ).rejects.toMatchObject({ name: 'AbortError' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('generates fresh cryptographic keys only on explicit calls and hides RNG failures', () => {
    const keys = Array.from({ length: 3 }, () => createAssistantIdempotencyKey());
    for (const key of keys) expect(key).toMatch(/^[0-9a-f]{64}$/u);
    expect(new Set(keys).size).toBe(3);
    const random = vi.spyOn(globalThis.crypto, 'getRandomValues').mockImplementation(() => {
      throw new Error('private random failure');
    });
    try {
      expect(() => createAssistantIdempotencyKey()).toThrow(
        'API transport failed with invalid_configuration.',
      );
    } finally {
      random.mockRestore();
    }
  });
});
