import type { ApiClient, ProductHealthHistoryResult } from '@stackplore/api-client';
import type { ProductApi } from './product-api-types';
import {
  hasExactKeys,
  isObservationMetadata,
  isRecord,
  isTimestamp,
  pageQuery,
  readQuery,
  resourceIdPattern,
  timeQuery,
  tokenPattern,
} from './product-api-common';

const microTime = (value: string) =>
  BigInt(Date.parse(value)) * 1000n +
  BigInt((value.match(/\.(\d{1,6})Z$/u)?.[1] ?? '').padEnd(6, '0').slice(3));
export function createHealthHistoryApi(client: ApiClient): Pick<ProductApi, 'listHealthHistory'> {
  return {
    async listHealthHistory(options) {
      if (
        !resourceIdPattern.test(options.resourceId) ||
        (options.minutes !== undefined &&
          (!Number.isInteger(options.minutes) || options.minutes < 5 || options.minutes > 43200)) ||
        (options.minutes !== undefined && (options.from !== undefined || options.to !== undefined))
      )
        throw new Error('Health history selection is invalid.');
      const limit = pageQuery(options, 200).limit;
      const response = await client.request<unknown>({
        path: '/health/history',
        query: {
          ...readQuery(options),
          limit,
          ...(options.from === undefined && options.to === undefined
            ? { minutes: options.minutes ?? 60 }
            : timeQuery(options, 30)),
        },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      const value = response.data;
      if (
        !isRecord(value) ||
        !hasExactKeys(
          value,
          ['availability', 'completeness', 'observed_at', 'items'],
          ['reason_code'],
        ) ||
        !isObservationMetadata(value) ||
        !Array.isArray(value.items) ||
        value.items.length > limit ||
        (!['available', 'stale'].includes(String(value.availability)) && value.items.length !== 0)
      )
        throw new Error('Health history response is invalid.');
      const seen = new Set<string>();
      let previous: { at: bigint; id: string } | undefined;
      for (const item of value.items) {
        if (
          !isRecord(item) ||
          !hasExactKeys(
            item,
            [
              'id',
              'resource_id',
              'state',
              'availability',
              'completeness',
              'observed_at',
              'synthetic',
            ],
            ['reason_code'],
          ) ||
          typeof item.id !== 'string' ||
          !tokenPattern.test(item.id) ||
          item.resource_id !== options.resourceId ||
          !['healthy', 'degraded', 'unhealthy', 'unknown'].includes(String(item.state)) ||
          !isObservationMetadata(item) ||
          typeof item.synthetic !== 'boolean' ||
          seen.has(item.id) ||
          ((!['available', 'stale'].includes(String(item.availability)) ||
            item.completeness === 'partial') &&
            item.state !== 'unknown') ||
          !isTimestamp(item.observed_at)
        )
          throw new Error('Health history sample is invalid.');
        const at = microTime(item.observed_at);
        if (
          at > microTime(String(value.observed_at)) ||
          (previous !== undefined &&
            (at > previous.at || (at === previous.at && item.id < previous.id))) ||
          (options.from !== undefined && at < microTime(options.from)) ||
          (options.to !== undefined && at >= microTime(options.to))
        )
          throw new Error('Health history order is invalid.');
        previous = { at, id: item.id };
        seen.add(item.id);
      }
      return value as ProductHealthHistoryResult;
    },
  };
}
