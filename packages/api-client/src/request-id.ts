const requestIdPattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;

export type RequestIdFactory = () => string;

export const isRequestId = (value: unknown): value is string =>
  typeof value === 'string' && requestIdPattern.test(value);

export const createRequestId = (): string => {
  const cryptoSource: unknown = globalThis.crypto;
  if (
    cryptoSource === null ||
    typeof cryptoSource !== 'object' ||
    !('randomUUID' in cryptoSource) ||
    typeof cryptoSource.randomUUID !== 'function'
  ) {
    throw new Error('A secure request identifier source is unavailable.');
  }

  let uuid: unknown;
  try {
    uuid = cryptoSource.randomUUID.call(cryptoSource);
  } catch {
    throw new Error('A secure request identifier source is unavailable.');
  }
  const value = typeof uuid === 'string' ? `req_${uuid}` : undefined;
  if (!isRequestId(value)) {
    throw new Error('A secure request identifier source is unavailable.');
  }
  return value;
};
