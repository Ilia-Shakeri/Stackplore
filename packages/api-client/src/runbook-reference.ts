import { ApiTransportError } from './client';
import type { RunbookReference } from './derived/types.derived';

const fields = ['corpus_id', 'version', 'digest', 'document_id', 'locale', 'generation'] as const;
const fail = (): never => {
  throw new ApiTransportError('unexpected_response');
};
const id = (value: unknown): string =>
  typeof value === 'string' && /^[a-z][a-z0-9_-]{0,63}$(?![\s\S])/u.test(value) ? value : fail();
const integer = (value: unknown, minimum: number): number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum ? value : fail();

// Shared by document transport and answer witnesses, without a response-parser cycle.
export function parseRunbookReference(value: unknown): RunbookReference {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return fail();
  const prototype: unknown = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) return fail();
  const descriptors = Object.getOwnPropertyDescriptors(value);
  if (Reflect.ownKeys(descriptors).length !== fields.length) return fail();
  const item: Record<string, unknown> = {};
  for (const field of fields) {
    const descriptor = descriptors[field];
    if (descriptor === undefined || !Object.hasOwn(descriptor, 'value')) return fail();
    item[field] = descriptor.value as unknown;
  }
  if (
    typeof item.digest !== 'string' ||
    !/^[a-f0-9]{64}$(?![\s\S])/u.test(item.digest) ||
    item.digest === '0'.repeat(64)
  )
    return fail();
  if (item.locale !== 'en' && item.locale !== 'fa') return fail();
  return {
    corpus_id: id(item.corpus_id),
    version: integer(item.version, 1),
    digest: item.digest,
    document_id: id(item.document_id),
    locale: item.locale,
    generation: integer(item.generation, 0),
  };
}
