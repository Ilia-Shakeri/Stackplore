import { describe, expect, it, vi } from 'vitest';

import {
  ApiProblemError,
  ApiTransportError,
  createApiClient,
  createRequestId,
  parseProblem,
} from './index';

const requestId = 'req_demo_01';

const jsonResponse = (
  value: unknown,
  init: ResponseInit & { readonly requestId?: string } = {},
) => {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json; charset=utf-8');
  if (init.requestId !== undefined) {
    headers.set('X-Request-ID', init.requestId);
  }
  return new Response(JSON.stringify(value), { ...init, headers });
};

describe('API client', () => {
  it('creates request identifiers that always meet the public form', () => {
    expect(createRequestId()).toMatch(/^req_[A-Fa-f0-9-]{36}$/u);
  });

  it('uses the fixed same-origin transport policy and encodes repeated filters', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockResolvedValue(jsonResponse({ items: [] }, { requestId, status: 200 }));
    const client = createApiClient({
      fetch: fetchMock,
      requestIdFactory: () => requestId,
    });

    const result = await client.request<{ readonly items: readonly unknown[] }>({
      path: '/resources',
      query: {
        kind: ['container', 'pod'],
        limit: 20,
      },
    });

    expect(result).toEqual({ data: { items: [] }, requestId, status: 200 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe('/api/v1/resources?kind=container&kind=pod&limit=20');
    expect(init).toMatchObject({
      cache: 'no-store',
      credentials: 'same-origin',
      method: 'GET',
      mode: 'same-origin',
      redirect: 'error',
      referrerPolicy: 'no-referrer',
    });
    const headers = new Headers(init?.headers);
    expect(headers.get('Accept')).toBe('application/json, application/problem+json');
    expect(headers.get('X-Request-ID')).toBe(requestId);
  });

  it('sends bounded JSON with safe caller headers', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockResolvedValue(
      new Response(null, { headers: { 'X-Request-ID': requestId }, status: 204 }),
    );
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await client.request<undefined, { readonly name: string }>({
      body: { name: 'demo' },
      headers: { 'If-Match': 'version_demo_01' },
      method: 'POST',
      path: '/resources',
    });

    const init = fetchMock.mock.calls[0]?.[1];
    expect(init?.body).toBe('{"name":"demo"}');
    const headers = new Headers(init?.headers);
    expect(headers.get('Content-Type')).toBe('application/json');
    expect(headers.get('If-Match')).toBe('version_demo_01');
  });

  it('requires a valid response request identifier on success and no-content replies', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    fetchMock.mockResolvedValueOnce(jsonResponse({ items: [] }, { status: 200 }));
    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'unexpected_response',
      requestId: null,
      status: 200,
    });

    fetchMock.mockResolvedValueOnce(
      new Response(null, { headers: { 'X-Request-ID': 'bad id' }, status: 204 }),
    );
    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'unexpected_response',
      requestId: null,
      status: 204,
    });
  });

  it('rejects a request body above the byte limit before fetch', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(
      client.request({
        body: { value: 'x'.repeat(256 * 1024) },
        method: 'POST',
        path: '/resources',
      }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('parses a sealed problem without putting detail in the thrown message', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const response = jsonResponse(
      {
        code: 'validation_failed',
        detail: 'Safe field detail.',
        invalid_parameters: [{ code: 'out_of_range', location: 'query', name: 'limit' }],
        request_id: requestId,
        status: 400,
        title: 'Invalid request',
        type: 'urn:stackplore:problem:validation_failed',
      },
      { requestId, status: 400 },
    );
    response.headers.set('Content-Type', 'application/problem+json; charset=utf-8');
    fetchMock.mockResolvedValue(response);
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client.request({ path: '/resources' }).catch((reason: unknown) => reason);

    expect(error).toBeInstanceOf(ApiProblemError);
    expect(error).toMatchObject({
      message: 'Request failed with validation_failed.',
      problem: { code: 'validation_failed', request_id: requestId, status: 400 },
    });
    expect((error as Error).message).not.toContain('Safe field detail');
  });

  it('rejects non-error status classes in direct problem parsing', () => {
    const makeProblem = (status: number) => ({
      code: 'unexpected_status',
      request_id: requestId,
      status,
      title: 'Unexpected status',
      type: 'urn:stackplore:problem:unexpected_status',
    });

    expect(parseProblem(makeProblem(200), 200, requestId)).toBeNull();
    expect(parseProblem(makeProblem(302), 302, requestId)).toBeNull();
    expect(parseProblem(makeProblem(400), 200, requestId)).toBeNull();
  });

  it('counts problem text by code point and rejects unsafe display text', () => {
    const makeProblem = (overrides: Record<string, unknown> = {}) => ({
      code: 'validation_failed',
      detail: '😀'.repeat(1024),
      invalid_parameters: [{ code: 'invalid', location: 'query', name: '😀'.repeat(128) }],
      request_id: requestId,
      status: 400,
      title: '😀'.repeat(128),
      type: 'urn:stackplore:problem:validation_failed',
      ...overrides,
    });

    expect(parseProblem(makeProblem(), 400, requestId)).not.toBeNull();
    expect(parseProblem(makeProblem({ title: '😀'.repeat(129) }), 400, requestId)).toBeNull();
    expect(parseProblem(makeProblem({ title: `bad\uD800` }), 400, requestId)).toBeNull();
    expect(parseProblem(makeProblem({ title: ' outer ' }), 400, requestId)).toBeNull();
    expect(parseProblem(makeProblem({ detail: 'safe\u202Etext' }), 400, requestId)).toBeNull();
    expect(
      parseProblem(
        makeProblem({
          invalid_parameters: [{ code: 'invalid', location: 'query', name: 'bad\nname' }],
        }),
        400,
        requestId,
      ),
    ).toBeNull();
    expect(
      parseProblem(makeProblem({ detail: '', title: 'safe\u200Dtitle' }), 400, requestId),
    ).not.toBeNull();
  });

  it('rejects malformed problem fields without echoing the body', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const privateText = 'private-input-must-not-escape';
    const response = jsonResponse(
      {
        code: 'bad',
        request_id: requestId,
        status: 400,
        title: 'Invalid request',
        type: 'urn:stackplore:problem:bad',
        unknown: privateText,
      },
      { requestId, status: 400 },
    );
    response.headers.set('Content-Type', 'application/problem+json');
    fetchMock.mockResolvedValue(response);
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client.request({ path: '/resources' }).catch((reason: unknown) => reason);

    expect(error).toBeInstanceOf(ApiTransportError);
    expect(error).toMatchObject({ code: 'invalid_problem', requestId, status: 400 });
    expect((error as Error).message).not.toContain(privateText);
  });

  it('requires a valid matching response request identifier for problems', async () => {
    const makeProblemResponse = (headerRequestId?: string) => {
      const response = jsonResponse(
        {
          code: 'forbidden',
          request_id: requestId,
          status: 403,
          title: 'Request not allowed',
          type: 'urn:stackplore:problem:forbidden',
        },
        { ...(headerRequestId === undefined ? {} : { requestId: headerRequestId }), status: 403 },
      );
      response.headers.set('Content-Type', 'application/problem+json');
      return response;
    };
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    fetchMock.mockResolvedValueOnce(makeProblemResponse());
    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'invalid_problem',
      requestId: null,
      status: 403,
    });

    fetchMock.mockResolvedValueOnce(makeProblemResponse('bad id'));
    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'invalid_problem',
      requestId: null,
      status: 403,
    });

    fetchMock.mockResolvedValueOnce(makeProblemResponse('req_other_01'));
    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'invalid_problem',
      requestId: 'req_other_01',
      status: 403,
    });
  });

  it('rejects external paths, traversal, and blocked headers before fetch', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(client.request({ path: '//outside.example' })).rejects.toMatchObject({
      code: 'invalid_path',
    });
    await expect(client.request({ path: '/../settings' })).rejects.toMatchObject({
      code: 'invalid_path',
    });
    await expect(client.request({ path: '/./settings' })).rejects.toMatchObject({
      code: 'invalid_path',
    });
    await expect(client.request({ path: '/%252e%252e/settings' })).rejects.toMatchObject({
      code: 'invalid_path',
    });
    await expect(client.request({ path: '/resource%20name' })).rejects.toMatchObject({
      code: 'invalid_path',
    });
    await expect(client.request({ path: '/resource\nname' } as never)).rejects.toMatchObject({
      code: 'invalid_path',
    });
    await expect(
      client.request({ headers: { Authorization: 'blocked' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects invalid runtime methods, query values, names, and headers', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(
      client.request({ method: 'TRACE', path: '/resources' } as never),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ path: '/resources', query: { kind: { raw: true } } } as never),
    ).rejects.toMatchObject({ code: 'invalid_query' });
    await expect(
      client.request({ path: '/resources', query: { 'Bad Key': undefined } } as never),
    ).rejects.toMatchObject({ code: 'invalid_query' });
    await expect(
      client.request({ headers: { 'Bad Header': 'value' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Safe': 4 }, path: '/resources' } as never),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Forwarded-Port': '443' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Api-Key': 'blocked' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Access-Key': 'blocked' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Auth': 'blocked' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Secret': 'blocked' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Safe': 'bad\u0000value' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    await expect(
      client.request({ headers: { 'X-Safe': 'bad\u0085value' }, path: '/resources' }),
    ).rejects.toMatchObject({ code: 'invalid_request' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('wraps request and map getter faults without exposing source text', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });
    const privateText = 'private getter detail';
    const query = Object.defineProperty({}, 'source', {
      enumerable: true,
      get: () => {
        throw new Error(privateText);
      },
    });
    const headers = Object.defineProperty({}, 'X-Safe', {
      enumerable: true,
      get: () => {
        throw new Error(privateText);
      },
    });
    const request = Object.defineProperty({}, 'path', {
      enumerable: true,
      get: () => {
        throw new Error(privateText);
      },
    });

    for (const [value, code] of [
      [{ path: '/resources', query }, 'invalid_query'],
      [{ headers, path: '/resources' }, 'invalid_request'],
      [request, 'invalid_request'],
    ] as const) {
      const error = await client.request(value as never).catch((reason: unknown) => reason);
      expect(error).toMatchObject({ code });
      expect((error as Error).message).not.toContain(privateText);
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('maps invalid request identifier sources to a safe configuration error', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createApiClient({
      fetch: fetchMock,
      requestIdFactory: () => {
        throw new Error('private identifier source detail');
      },
    });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'invalid_configuration',
      requestId: null,
      status: null,
    });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(() => createApiClient(null as never)).toThrow(
      expect.objectContaining({ code: 'invalid_configuration' }),
    );
    expect(() => createApiClient({ fetch: fetchMock, requestIdFactory: 4 as never })).toThrow(
      expect.objectContaining({ code: 'invalid_configuration' }),
    );
    const unsafeOptions = Object.defineProperty({}, 'fetch', {
      get: () => {
        throw new Error('private option getter detail');
      },
    });
    expect(() => createApiClient(unsafeOptions as never)).toThrow(
      expect.objectContaining({ code: 'invalid_configuration' }),
    );
    expect(() =>
      createApiClient({ fetch: fetchMock, maximumResponseBytes: 64 * 1024 * 1024 }),
    ).not.toThrow();
    expect(() =>
      createApiClient({ fetch: fetchMock, maximumResponseBytes: 64 * 1024 * 1024 + 1 }),
    ).toThrow(expect.objectContaining({ code: 'invalid_configuration' }));
  });

  it('does not retry a network failure', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockRejectedValue(new Error('network detail'));
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'network_error',
      requestId,
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('maps cancellation to a fresh safe abort error', async () => {
    const controller = new AbortController();
    const privateText = 'private abort source';
    const abortError = new DOMException(privateText, 'AbortError');
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockImplementation(() => {
      controller.abort();
      return Promise.reject(abortError);
    });
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client
      .request({ path: '/resources', signal: controller.signal })
      .catch((reason: unknown) => reason);

    expect(error).not.toBe(abortError);
    expect(error).toMatchObject({ message: 'The request was aborted.', name: 'AbortError' });
    expect((error as Error).message).not.toContain(privateText);
  });

  it('maps a custom abort reason to a fresh safe abort error', async () => {
    const controller = new AbortController();
    const privateReason = new Error('private custom abort reason');
    controller.abort(privateReason);
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockRejectedValue(privateReason);
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client
      .request({ path: '/resources', signal: controller.signal })
      .catch((reason: unknown) => reason);

    expect(error).not.toBe(privateReason);
    expect(error).toMatchObject({ message: 'The request was aborted.', name: 'AbortError' });
    expect((error as Error).message).not.toContain(privateReason.message);
  });

  it('wraps an ordinary private fetch fault even when the signal is aborted', async () => {
    const controller = new AbortController();
    const privateText = 'private non-abort fetch fault';
    controller.abort();
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockRejectedValue(new Error(privateText));
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client
      .request({ path: '/resources', signal: controller.signal })
      .catch((reason: unknown) => reason);

    expect(error).toMatchObject({ code: 'network_error', requestId });
    expect((error as Error).message).not.toContain(privateText);
  });

  it('rejects a response above the configured byte bound', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockResolvedValue(
      new Response('{"ok":true}', {
        headers: {
          'Content-Length': '2048',
          'Content-Type': 'application/json',
          'X-Request-ID': requestId,
        },
        status: 200,
      }),
    );
    const client = createApiClient({
      fetch: fetchMock,
      maximumResponseBytes: 1024,
      requestIdFactory: () => requestId,
    });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'response_too_large',
      status: 200,
    });
  });

  it('rejects a malformed content length before reading the body', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockResolvedValue(
      new Response('{"ok":true}', {
        headers: {
          'Content-Length': '1e3',
          'Content-Type': 'application/json',
          'X-Request-ID': requestId,
        },
        status: 200,
      }),
    );
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'unexpected_response',
      requestId,
      status: 200,
    });
  });

  it('applies a tighter bound to problem bodies', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockResolvedValue(
      new Response('{}', {
        headers: {
          'Content-Length': String(65 * 1024),
          'Content-Type': 'application/problem+json',
          'X-Request-ID': requestId,
        },
        status: 400,
      }),
    );
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'response_too_large',
      requestId,
      status: 400,
    });
  });

  it('wraps response stream faults without exposing source text', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const privateText = 'stream-private-source';
    const abortController = new AbortController();
    const stream = new ReadableStream<Uint8Array>({
      pull() {
        abortController.abort();
        throw new Error(privateText);
      },
    });
    fetchMock.mockResolvedValue(
      new Response(stream, {
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId },
        status: 200,
      }),
    );
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client
      .request({ path: '/resources', signal: abortController.signal })
      .catch((reason: unknown) => reason);

    expect(error).toMatchObject({ code: 'unexpected_response', requestId, status: 200 });
    expect((error as Error).message).not.toContain(privateText);
  });

  it('wraps response stream cancellation faults', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const abortController = new AbortController();
    const privateText = 'cancel-private-source';
    const stream = new ReadableStream<Uint8Array>({
      cancel() {
        abortController.abort();
        throw new Error(privateText);
      },
      start(controller) {
        controller.enqueue(new Uint8Array(2048));
      },
    });
    fetchMock.mockResolvedValue(
      new Response(stream, {
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId },
        status: 200,
      }),
    );
    const client = createApiClient({
      fetch: fetchMock,
      maximumResponseBytes: 1024,
      requestIdFactory: () => requestId,
    });

    const error = await client
      .request({ path: '/resources', signal: abortController.signal })
      .catch((reason: unknown) => reason);

    expect(error).toMatchObject({ code: 'unexpected_response', requestId, status: 200 });
    expect((error as Error).message).not.toContain(privateText);
  });

  it('wraps response reader lock faults after an aborted signal', async () => {
    const abortController = new AbortController();
    const privateText = 'release-lock-private-source';
    const fetchMock = vi.fn<typeof fetch>();
    const response = {
      body: {
        getReader: () => ({
          cancel: vi.fn(),
          read: () => Promise.resolve({ done: true, value: undefined }),
          releaseLock: () => {
            abortController.abort();
            throw new Error(privateText);
          },
        }),
      },
      headers: new Headers({ 'Content-Type': 'application/json', 'X-Request-ID': requestId }),
      ok: true,
      status: 200,
    } as unknown as Response;
    fetchMock.mockResolvedValue(response);
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    const error = await client
      .request({ path: '/resources', signal: abortController.signal })
      .catch((reason: unknown) => reason);

    expect(error).toMatchObject({ code: 'unexpected_response', requestId, status: 200 });
    expect((error as Error).message).not.toContain(privateText);
  });

  it('rejects a streamed response when chunks cross the byte limit', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const cancel = vi.fn();
    fetchMock.mockResolvedValue(
      new Response(
        new ReadableStream<Uint8Array>({
          cancel,
          start(controller) {
            controller.enqueue(new Uint8Array(700));
            controller.enqueue(new Uint8Array(700));
          },
        }),
        {
          headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId },
          status: 200,
        },
      ),
    );
    const client = createApiClient({
      fetch: fetchMock,
      maximumResponseBytes: 1024,
      requestIdFactory: () => requestId,
    });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'response_too_large',
      requestId,
      status: 200,
    });
    expect(cancel).toHaveBeenCalledTimes(1);
  });

  it('keeps the response request identifier on invalid UTF-8', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockResolvedValue(
      new Response(new Uint8Array([0xc3, 0x28]), {
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId },
        status: 200,
      }),
    );
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'unexpected_response',
      requestId,
      status: 200,
    });
  });

  it('rejects an unexpected successful media type and cancels its body', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const cancel = vi.fn();
    fetchMock.mockResolvedValue(
      new Response(
        new ReadableStream<Uint8Array>({
          cancel,
          start(controller) {
            controller.enqueue(new TextEncoder().encode('plain body'));
          },
        }),
        {
          headers: { 'Content-Type': 'text/plain', 'X-Request-ID': requestId },
          status: 200,
        },
      ),
    );
    const client = createApiClient({ fetch: fetchMock, requestIdFactory: () => requestId });

    await expect(client.request({ path: '/resources' })).rejects.toMatchObject({
      code: 'unexpected_response',
      requestId,
      status: 200,
    });
    expect(cancel).toHaveBeenCalledTimes(1);
  });
});
