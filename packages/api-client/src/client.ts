import { ApiProblemError, parseProblem } from './problem';
import { createRequestId, isRequestId, type RequestIdFactory } from './request-id';

const apiBasePath = '/api/v1';
const defaultMaximumResponseBytes = 1024 * 1024;
const maximumAllowedResponseBytes = 64 * 1024 * 1024;
const maximumProblemBytes = 64 * 1024;
const maximumRequestBytes = 256 * 1024;
const maximumUrlLength = 8192;
const maximumHeaderCount = 32;
const maximumHeaderValueLength = 2048;
const contentLengthPattern = /^(?:0|[1-9][0-9]*)$/u;
const protectionTokenPattern = /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;

const blockedHeaders = new Set([
  'accept',
  'authorization',
  'connection',
  'content-length',
  'content-type',
  'cookie',
  'expect',
  'forwarded',
  'host',
  'keep-alive',
  'origin',
  'proxy-authorization',
  'referer',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
  'via',
  'x-http-method-override',
  'x-real-ip',
  'x-forwarded-for',
  'x-forwarded-host',
  'x-forwarded-proto',
  'x-request-id',
  'x-stackplore-csrf',
  'x-stackplore-preauth',
  'x-stackplore-recent-auth',
]);

const queryNamePattern = /^[a-z][a-z0-9_]{0,63}$/u;
const visibleAsciiPathPattern = /^[\u0021-\u007E]+$/u;
const headerNamePattern = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/u;
const headerValueControlPattern = /\p{Cc}/u;
const sensitiveHeaderFragments = [
  'accesskey',
  'apikey',
  'authorization',
  'cookie',
  'credential',
  'password',
  'privatekey',
  'secret',
  'session',
  'token',
];
const methods = new Set<ApiMethod>(['DELETE', 'GET', 'PATCH', 'POST', 'PUT']);

export type ApiMethod = 'DELETE' | 'GET' | 'PATCH' | 'POST' | 'PUT';
export type QueryPrimitive = boolean | number | string;
export type ApiQuery = Readonly<
  Record<string, QueryPrimitive | readonly QueryPrimitive[] | undefined>
>;

export type ApiRequestProtection =
  | {
      readonly kind: 'preauth';
      readonly token: string;
    }
  | {
      readonly csrfToken: string;
      readonly kind: 'session';
      readonly recentAuthToken?: string;
    };

export interface ApiRequest<TBody = never> {
  readonly body?: TBody;
  readonly headers?: Readonly<Record<string, string>>;
  readonly method?: ApiMethod;
  readonly path: `/${string}`;
  readonly protection?: ApiRequestProtection;
  readonly query?: ApiQuery;
  readonly signal?: AbortSignal;
}

export interface ApiResponse<TData> {
  readonly data: TData;
  readonly requestId: string;
  readonly status: number;
}

export interface ApiClient {
  request<TData, TBody = never>(request: ApiRequest<TBody>): Promise<ApiResponse<TData>>;
}

export interface ApiClientOptions {
  readonly fetch?: typeof globalThis.fetch;
  readonly maximumResponseBytes?: number;
  readonly requestIdFactory?: RequestIdFactory;
}

export type ApiTransportErrorCode =
  | 'invalid_configuration'
  | 'invalid_path'
  | 'invalid_problem'
  | 'invalid_query'
  | 'invalid_request'
  | 'network_error'
  | 'response_too_large'
  | 'unexpected_response';

export class ApiTransportError extends Error {
  readonly code: ApiTransportErrorCode;
  readonly requestId: string | null;
  readonly status: number | null;

  constructor(
    code: ApiTransportErrorCode,
    options: { readonly requestId?: string | null; readonly status?: number | null } = {},
  ) {
    super(`API transport failed with ${code}.`);
    this.name = 'ApiTransportError';
    this.code = code;
    this.requestId = options.requestId ?? null;
    this.status = options.status ?? null;
  }
}

const isAbortError = (error: unknown) =>
  (error instanceof DOMException || error instanceof Error) && error.name === 'AbortError';

const isTrueAbort = (error: unknown, signal: AbortSignal | undefined) =>
  isAbortError(error) || (signal?.aborted === true && error === signal.reason);

const throwSafeAbort = (): never => {
  throw new DOMException('The request was aborted.', 'AbortError');
};

const throwResponseFault = (
  error: unknown,
  response: Response,
  signal: AbortSignal | undefined,
): never => {
  if (isTrueAbort(error, signal)) {
    throwSafeAbort();
  }
  throw new ApiTransportError('unexpected_response', {
    requestId: readResponseRequestId(response),
    status: response.status,
  });
};

const assertMaximumResponseBytes: (value: unknown) => asserts value is number = (value) => {
  if (
    typeof value !== 'number' ||
    !Number.isInteger(value) ||
    value < 1024 ||
    value > maximumAllowedResponseBytes
  ) {
    throw new ApiTransportError('invalid_configuration');
  }
};

const readObjectEntries = (
  value: object,
  code: 'invalid_query' | 'invalid_request',
): [string, unknown][] => {
  try {
    return Object.entries(value);
  } catch {
    throw new ApiTransportError(code);
  }
};

const readOwnKeys = (value: object): readonly PropertyKey[] => {
  try {
    return Reflect.ownKeys(value);
  } catch {
    throw new ApiTransportError('invalid_request');
  }
};

const buildPath = (path: unknown, query: unknown) => {
  if (
    typeof path !== 'string' ||
    !path.startsWith('/') ||
    path.startsWith('//') ||
    path.includes('?') ||
    path.includes('#') ||
    path.includes('\\') ||
    path.includes('%') ||
    !visibleAsciiPathPattern.test(path) ||
    path.split('/').some((part) => part === '.' || part === '..')
  ) {
    throw new ApiTransportError('invalid_path');
  }

  const parameters = new URLSearchParams();
  const rawQuery: unknown = query;
  if (rawQuery !== undefined) {
    if (rawQuery === null || typeof rawQuery !== 'object' || Array.isArray(rawQuery)) {
      throw new ApiTransportError('invalid_query');
    }
    const entries = readObjectEntries(rawQuery, 'invalid_query');
    if (entries.length > 64) {
      throw new ApiTransportError('invalid_query');
    }

    for (const [name, rawValue] of entries) {
      if (!queryNamePattern.test(name)) {
        throw new ApiTransportError('invalid_query');
      }
      if (rawValue === undefined) {
        continue;
      }

      const values = Array.isArray(rawValue) ? rawValue : [rawValue];
      if (values.length === 0 || values.length > 64) {
        throw new ApiTransportError('invalid_query');
      }

      for (const value of values) {
        if (
          (typeof value !== 'boolean' && typeof value !== 'number' && typeof value !== 'string') ||
          (typeof value === 'number' && !Number.isFinite(value)) ||
          (typeof value === 'string' && value.length > 2048)
        ) {
          throw new ApiTransportError('invalid_query');
        }
        parameters.append(name, String(value));
      }
    }
  }

  const search = parameters.toString();
  const url = `${apiBasePath}${path}${search.length === 0 ? '' : `?${search}`}`;
  if (url.length > maximumUrlLength) {
    throw new ApiTransportError('invalid_query');
  }
  return url;
};

const addProtectionHeaders = (headers: Headers, rawProtection: unknown, method: ApiMethod) => {
  if (rawProtection === undefined) {
    return;
  }
  if (
    method === 'GET' ||
    rawProtection === null ||
    typeof rawProtection !== 'object' ||
    Array.isArray(rawProtection)
  ) {
    throw new ApiTransportError('invalid_request');
  }

  const entries = readObjectEntries(rawProtection, 'invalid_request');
  const ownKeys = readOwnKeys(rawProtection);
  if (ownKeys.length !== entries.length || ownKeys.some((key) => typeof key !== 'string')) {
    throw new ApiTransportError('invalid_request');
  }
  const values = new Map(entries);
  const kind = values.get('kind');
  if (kind === 'preauth') {
    const token = values.get('token');
    if (
      entries.length !== 2 ||
      !values.has('token') ||
      typeof token !== 'string' ||
      !protectionTokenPattern.test(token)
    ) {
      throw new ApiTransportError('invalid_request');
    }
    headers.set('X-Stackplore-Preauth', token);
    return;
  }

  if (kind === 'session') {
    const csrfToken = values.get('csrfToken');
    const recentAuthToken = values.get('recentAuthToken');
    if (
      (entries.length !== 2 && entries.length !== 3) ||
      !values.has('csrfToken') ||
      entries.some(
        ([name]) => name !== 'kind' && name !== 'csrfToken' && name !== 'recentAuthToken',
      ) ||
      typeof csrfToken !== 'string' ||
      !protectionTokenPattern.test(csrfToken) ||
      (values.has('recentAuthToken') &&
        (typeof recentAuthToken !== 'string' || !protectionTokenPattern.test(recentAuthToken)))
    ) {
      throw new ApiTransportError('invalid_request');
    }
    headers.set('X-Stackplore-CSRF', csrfToken);
    if (typeof recentAuthToken === 'string') {
      headers.set('X-Stackplore-Recent-Auth', recentAuthToken);
    }
    return;
  }

  throw new ApiTransportError('invalid_request');
};

const buildHeaders = (
  requestHeaders: unknown,
  rawProtection: unknown,
  requestId: string,
  hasBody: boolean,
  method: ApiMethod,
) => {
  const headers = new Headers({
    Accept: 'application/json, application/problem+json',
    'X-Request-ID': requestId,
  });

  const rawRequestHeaders: unknown = requestHeaders;
  if (
    rawRequestHeaders !== undefined &&
    (rawRequestHeaders === null ||
      typeof rawRequestHeaders !== 'object' ||
      Array.isArray(rawRequestHeaders))
  ) {
    throw new ApiTransportError('invalid_request');
  }
  const entries = readObjectEntries(rawRequestHeaders ?? {}, 'invalid_request');
  if (entries.length > maximumHeaderCount) {
    throw new ApiTransportError('invalid_request');
  }

  for (const [name, value] of entries) {
    const normalizedName = name.toLowerCase();
    const compactName = normalizedName.replaceAll(/[^a-z0-9]/gu, '');
    if (
      !headerNamePattern.test(name) ||
      typeof value !== 'string' ||
      blockedHeaders.has(normalizedName) ||
      normalizedName.startsWith('proxy-') ||
      normalizedName.startsWith('sec-') ||
      normalizedName.startsWith('x-forwarded-') ||
      compactName.endsWith('auth') ||
      sensitiveHeaderFragments.some((fragment) => compactName.includes(fragment)) ||
      value.length > maximumHeaderValueLength ||
      headerValueControlPattern.test(value)
    ) {
      throw new ApiTransportError('invalid_request');
    }
    try {
      headers.set(name, value);
    } catch {
      throw new ApiTransportError('invalid_request');
    }
  }

  if (hasBody) {
    headers.set('Content-Type', 'application/json');
  }
  addProtectionHeaders(headers, rawProtection, method);
  return headers;
};

const readResponseRequestId = (response: Response) => {
  const requestId = response.headers.get('X-Request-ID');
  return isRequestId(requestId) ? requestId : null;
};

const cancelResponseBody = async (response: Response, signal: AbortSignal | undefined) => {
  if (response.body === null) {
    return;
  }
  try {
    await response.body.cancel();
  } catch (error) {
    throwResponseFault(error, response, signal);
  }
};

const readBoundedBody = async (
  response: Response,
  maximumBytes: number,
  signal: AbortSignal | undefined,
) => {
  const contentLength = response.headers.get('Content-Length');
  if (contentLength !== null) {
    const parsedLength = Number(contentLength);
    if (!contentLengthPattern.test(contentLength) || !Number.isSafeInteger(parsedLength)) {
      await cancelResponseBody(response, signal);
      throw new ApiTransportError('unexpected_response', {
        requestId: readResponseRequestId(response),
        status: response.status,
      });
    }
    if (parsedLength > maximumBytes) {
      await cancelResponseBody(response, signal);
      throw new ApiTransportError('response_too_large', {
        requestId: readResponseRequestId(response),
        status: response.status,
      });
    }
  }

  if (response.body === null) {
    return '';
  }

  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  let streamFailed = false;
  let streamError: unknown;

  try {
    let done = false;
    while (!done) {
      const result = await reader.read();
      if (result.done) {
        done = true;
        continue;
      }
      totalBytes += result.value.byteLength;
      if (totalBytes > maximumBytes) {
        try {
          await reader.cancel();
        } catch (error) {
          throwResponseFault(error, response, signal);
        }
        throw new ApiTransportError('response_too_large', {
          requestId: readResponseRequestId(response),
          status: response.status,
        });
      }
      chunks.push(result.value);
    }
  } catch (error) {
    streamFailed = true;
    streamError = error;
  }

  try {
    reader.releaseLock();
  } catch (error) {
    throwResponseFault(error, response, signal);
  }

  if (streamFailed) {
    if (streamError instanceof ApiTransportError) {
      throw streamError;
    }
    throwResponseFault(streamError, response, signal);
  }

  const bytes = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }

  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    throw new ApiTransportError('unexpected_response', {
      requestId: readResponseRequestId(response),
      status: response.status,
    });
  }
};

const parseJson = (text: string, response: Response, code: ApiTransportErrorCode) => {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new ApiTransportError(code, {
      requestId: readResponseRequestId(response),
      status: response.status,
    });
  }
};

const readMediaType = (response: Response) =>
  response.headers.get('Content-Type')?.split(';', 1)[0]?.trim().toLowerCase() ?? '';

const isJsonMediaType = (mediaType: string) =>
  mediaType === 'application/json' || /^application\/[a-z0-9!#$&^_.+-]+\+json$/u.test(mediaType);

export const createApiClient = (options: ApiClientOptions = {}): ApiClient => {
  const rawOptions: unknown = options;
  if (rawOptions === null || typeof rawOptions !== 'object' || Array.isArray(rawOptions)) {
    throw new ApiTransportError('invalid_configuration');
  }

  let fetchOption: typeof globalThis.fetch | undefined;
  let maximumResponseBytesOption: number | undefined;
  let requestIdFactoryOption: RequestIdFactory | undefined;
  try {
    fetchOption = options.fetch;
    maximumResponseBytesOption = options.maximumResponseBytes;
    requestIdFactoryOption = options.requestIdFactory;
  } catch {
    throw new ApiTransportError('invalid_configuration');
  }

  const fetchImplementation = fetchOption ?? globalThis.fetch;
  if (typeof fetchImplementation !== 'function') {
    throw new ApiTransportError('invalid_configuration');
  }

  const maximumResponseBytes = maximumResponseBytesOption ?? defaultMaximumResponseBytes;
  assertMaximumResponseBytes(maximumResponseBytes);
  const requestIdFactoryValue: unknown = requestIdFactoryOption ?? createRequestId;
  if (typeof requestIdFactoryValue !== 'function') {
    throw new ApiTransportError('invalid_configuration');
  }
  const requestIdFactory = requestIdFactoryValue as RequestIdFactory;

  return {
    async request<TData, TBody = never>(request: ApiRequest<TBody>): Promise<ApiResponse<TData>> {
      const rawRequest: unknown = request;
      if (rawRequest === null || typeof rawRequest !== 'object' || Array.isArray(rawRequest)) {
        throw new ApiTransportError('invalid_request');
      }

      let requestBody: TBody | undefined;
      let requestHeaders: Readonly<Record<string, string>> | undefined;
      let requestMethod: ApiMethod | undefined;
      let requestPath: `/${string}`;
      let requestProtection: ApiRequestProtection | undefined;
      let requestQuery: ApiQuery | undefined;
      let requestSignal: AbortSignal | undefined;
      try {
        requestBody = request.body;
        requestHeaders = request.headers;
        requestMethod = request.method;
        requestPath = request.path;
        requestProtection = request.protection;
        requestQuery = request.query;
        requestSignal = request.signal;
      } catch {
        throw new ApiTransportError('invalid_request');
      }

      const rawMethod: unknown = requestMethod ?? 'GET';
      if (typeof rawMethod !== 'string' || !methods.has(rawMethod as ApiMethod)) {
        throw new ApiTransportError('invalid_request');
      }
      const method = rawMethod as ApiMethod;
      const hasBody = requestBody !== undefined;
      if (hasBody && method === 'GET') {
        throw new ApiTransportError('invalid_request');
      }

      let requestId: string;
      try {
        requestId = requestIdFactory();
      } catch {
        throw new ApiTransportError('invalid_configuration');
      }
      if (!isRequestId(requestId)) {
        throw new ApiTransportError('invalid_configuration');
      }

      const url = buildPath(requestPath, requestQuery);
      const headers = buildHeaders(requestHeaders, requestProtection, requestId, hasBody, method);
      let body: string | undefined;
      if (hasBody) {
        let serializedBody: unknown;
        try {
          serializedBody = JSON.stringify(requestBody);
        } catch {
          throw new ApiTransportError('invalid_request');
        }

        if (
          typeof serializedBody !== 'string' ||
          new TextEncoder().encode(serializedBody).byteLength > maximumRequestBytes
        ) {
          throw new ApiTransportError('invalid_request');
        }
        body = serializedBody;
      }

      const init: RequestInit = {
        cache: 'no-store',
        credentials: 'same-origin',
        headers,
        method,
        mode: 'same-origin',
        redirect: 'error',
        referrerPolicy: 'no-referrer',
        ...(body === undefined ? {} : { body }),
        ...(requestSignal === undefined ? {} : { signal: requestSignal }),
      };

      let response: Response;
      try {
        response = await fetchImplementation(url, init);
      } catch (error) {
        if (isTrueAbort(error, requestSignal)) {
          throwSafeAbort();
        }
        throw new ApiTransportError('network_error', { requestId });
      }

      const responseRequestId = readResponseRequestId(response);
      const mediaType = readMediaType(response);
      if (responseRequestId === null) {
        await cancelResponseBody(response, requestSignal);
        throw new ApiTransportError(
          !response.ok && mediaType === 'application/problem+json'
            ? 'invalid_problem'
            : 'unexpected_response',
          { status: response.status },
        );
      }

      if (response.status === 204 || response.status === 205) {
        await cancelResponseBody(response, requestSignal);
        return {
          data: undefined as TData,
          requestId: responseRequestId,
          status: response.status,
        };
      }

      if (response.ok) {
        if (!isJsonMediaType(mediaType) || mediaType === 'application/problem+json') {
          await cancelResponseBody(response, requestSignal);
          throw new ApiTransportError('unexpected_response', {
            requestId: responseRequestId,
            status: response.status,
          });
        }

        const text = await readBoundedBody(response, maximumResponseBytes, requestSignal);
        const data = parseJson(text, response, 'unexpected_response') as TData;
        return {
          data,
          requestId: responseRequestId,
          status: response.status,
        };
      }

      if (mediaType !== 'application/problem+json') {
        await cancelResponseBody(response, requestSignal);
        throw new ApiTransportError('unexpected_response', {
          requestId: responseRequestId,
          status: response.status,
        });
      }

      const text = await readBoundedBody(
        response,
        Math.min(maximumResponseBytes, maximumProblemBytes),
        requestSignal,
      );
      const value = parseJson(text, response, 'invalid_problem');
      const problem = parseProblem(value, response.status, responseRequestId);
      if (problem === null) {
        throw new ApiTransportError('invalid_problem', {
          requestId: responseRequestId,
          status: response.status,
        });
      }
      throw new ApiProblemError(problem);
    },
  };
};
