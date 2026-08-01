import type { InvalidParameter, Problem } from './derived/types.derived';
import { isRequestId } from './request-id';

const codePattern = /^[a-z][a-z0-9._-]{0,63}$/u;
const problemTypePattern = /^urn:stackplore:problem:[a-z][a-z0-9._-]{0,63}$/u;
const loneSurrogatePattern = /[\uD800-\uDFFF]/u;
const outerWhitespacePattern = /^\p{White_Space}|\p{White_Space}$/u;
const unsafeDisplayPattern = /[\p{Cc}\p{Zl}\p{Zp}]/u;
const formatCharacterPattern = /\p{Cf}/u;
const visibleBasePattern = /[\p{L}\p{N}\p{P}\p{S}]/u;
const parameterLocations = new Set(['body', 'header', 'path', 'query']);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

const hasOnlyKeys = (value: Record<string, unknown>, allowedKeys: ReadonlySet<string>) =>
  Object.keys(value).every((key) => allowedKeys.has(key));

const isBoundedString = (
  value: unknown,
  maximumLength: number,
  allowEmpty = false,
): value is string =>
  typeof value === 'string' &&
  !loneSurrogatePattern.test(value) &&
  Array.from(value).length <= maximumLength &&
  (allowEmpty || value.length > 0);

const isDisplayString = (
  value: unknown,
  maximumLength: number,
  allowEmpty = false,
): value is string => {
  if (!isBoundedString(value, maximumLength, allowEmpty)) {
    return false;
  }
  if (value.length === 0) {
    return allowEmpty;
  }
  if (outerWhitespacePattern.test(value) || !visibleBasePattern.test(value)) {
    return false;
  }
  return Array.from(value).every(
    (character) =>
      !unsafeDisplayPattern.test(character) &&
      (!formatCharacterPattern.test(character) || character === '\u200c' || character === '\u200d'),
  );
};

const parseInvalidParameter = (value: unknown): InvalidParameter | null => {
  if (!isRecord(value)) {
    return null;
  }

  const allowedKeys = new Set(['code', 'location', 'name']);
  if (
    !hasOnlyKeys(value, allowedKeys) ||
    typeof value.location !== 'string' ||
    !parameterLocations.has(value.location) ||
    !isDisplayString(value.name, 128) ||
    typeof value.code !== 'string' ||
    !codePattern.test(value.code)
  ) {
    return null;
  }

  return {
    code: value.code,
    location: value.location as InvalidParameter['location'],
    name: value.name,
  };
};

export const parseProblem = (
  value: unknown,
  expectedStatus: number,
  responseRequestId: string,
): Problem | null => {
  if (!isRecord(value)) {
    return null;
  }

  const allowedKeys = new Set([
    'code',
    'detail',
    'invalid_parameters',
    'request_id',
    'status',
    'title',
    'type',
  ]);
  if (
    !hasOnlyKeys(value, allowedKeys) ||
    typeof value.type !== 'string' ||
    !problemTypePattern.test(value.type) ||
    !isDisplayString(value.title, 128) ||
    !Number.isInteger(expectedStatus) ||
    expectedStatus < 400 ||
    expectedStatus > 599 ||
    typeof value.status !== 'number' ||
    !Number.isInteger(value.status) ||
    value.status < 400 ||
    value.status > 599 ||
    value.status !== expectedStatus ||
    typeof value.code !== 'string' ||
    !codePattern.test(value.code) ||
    !isRequestId(value.request_id) ||
    !isRequestId(responseRequestId) ||
    value.request_id !== responseRequestId ||
    (value.detail !== undefined && !isDisplayString(value.detail, 1024, true))
  ) {
    return null;
  }

  let invalidParameters: InvalidParameter[] | undefined;
  if (value.invalid_parameters !== undefined) {
    if (!Array.isArray(value.invalid_parameters) || value.invalid_parameters.length > 32) {
      return null;
    }

    invalidParameters = [];
    for (const parameter of value.invalid_parameters) {
      const parsed = parseInvalidParameter(parameter);
      if (parsed === null) {
        return null;
      }
      invalidParameters.push(parsed);
    }
  }

  return {
    code: value.code,
    request_id: value.request_id,
    status: value.status,
    title: value.title,
    type: value.type,
    ...(value.detail === undefined ? {} : { detail: value.detail }),
    ...(invalidParameters === undefined ? {} : { invalid_parameters: invalidParameters }),
  };
};

export class ApiProblemError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(`Request failed with ${problem.code}.`);
    this.name = 'ApiProblemError';
    this.problem = problem;
  }
}
