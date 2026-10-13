import {
  ApiProblemError,
  ApiTransportError,
  type ApiClient,
  type SourceSettingsPage,
  type SourceSettingsRecord,
  type SourcePreviewRequest,
  type SourceUpdatePreview,
  type SourceUpdateRequest,
} from '@stackplore/api-client';

import type { SessionActionRunner } from './product-api';

interface SourcePageOptions {
  readonly cursor?: string;
  readonly signal?: AbortSignal;
}

export interface SourceSettingsApi {
  readonly listSourceSettings: (options?: SourcePageOptions) => Promise<SourceSettingsPage>;
  readonly previewSourceUpdate: (
    source: SourceSettingsRecord,
    enabled: boolean,
    signal?: AbortSignal,
  ) => Promise<SourceUpdatePreview>;
  readonly updateSource: (
    preview: SourceUpdatePreview,
    currentPassword: string,
  ) => Promise<SourceSettingsRecord>;
}

export type SourceSettingsFailure =
  | 'stale'
  | 'confirmation'
  | 'profile'
  | 'denied'
  | 'session'
  | 'rate_limited'
  | 'unavailable'
  | 'uncertain'
  | 'invalid'
  | 'unexpected';

export class SourceSettingsUncertainError extends Error {
  constructor() {
    super('Source update outcome is unknown.');
    this.name = 'SourceSettingsUncertainError';
  }
}

export function classifySourceSettingsFailure(error: unknown): SourceSettingsFailure {
  if (error instanceof SourceSettingsUncertainError) return 'uncertain';
  if (error instanceof ApiProblemError) {
    if (
      error.problem.code === 'source.version_conflict' ||
      error.problem.code === 'source.state_unchanged'
    )
      return 'stale';
    if (error.problem.code === 'source.confirmation_rejected') return 'confirmation';
    if (error.problem.code === 'source.profile_unavailable') return 'profile';
    switch (error.problem.status) {
      case 400:
        return 'invalid';
      case 401:
        return 'session';
      case 403:
        return 'denied';
      case 429:
        return 'rate_limited';
      case 500:
      case 503:
        return 'unavailable';
      default:
        return 'unexpected';
    }
  }
  return error instanceof ApiTransportError && error.code === 'network_error'
    ? 'unavailable'
    : 'unexpected';
}

const idPattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const cursorPattern = /^[A-Za-z0-9_-]{1,4096}$/u;
const proofPattern = /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const timestampPattern =
  /^(?!0000)[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z$/u;
const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const exactKeys = (value: Record<string, unknown>, keys: readonly string[]) =>
  Object.keys(value).length === keys.length && keys.every((key) => Object.hasOwn(value, key));
const versionValid = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 1;
const displayNameValid = (value: unknown): value is string =>
  typeof value === 'string' &&
  Array.from(value).length >= 1 &&
  Array.from(value).length <= 256 &&
  value.trim() === value &&
  /[\p{L}\p{N}\p{P}\p{S}]/u.test(value) &&
  Array.from(value).every(
    (character) =>
      !/[\p{Cc}\p{Zl}\p{Zp}]/u.test(character) &&
      (!/\p{Cf}/u.test(character) || character === '\u200c' || character === '\u200d'),
  );

export const isSourceSettingsRecord = (value: unknown): value is SourceSettingsRecord =>
  isRecord(value) &&
  exactKeys(value, [
    'id',
    'provider',
    'name',
    'synthetic',
    'enabled',
    'version',
    'profile_available',
  ]) &&
  typeof value.id === 'string' &&
  idPattern.test(value.id) &&
  ['demo', 'docker', 'kubernetes'].includes(String(value.provider)) &&
  displayNameValid(value.name) &&
  typeof value.synthetic === 'boolean' &&
  typeof value.enabled === 'boolean' &&
  typeof value.profile_available === 'boolean' &&
  versionValid(value.version);

const parseSourcePage = (value: unknown): SourceSettingsPage => {
  if (
    !isRecord(value) ||
    !exactKeys(value, ['items', 'page']) ||
    !Array.isArray(value.items) ||
    value.items.length > 100 ||
    !value.items.every(isSourceSettingsRecord) ||
    !isRecord(value.page)
  )
    throw new Error('Source settings response is invalid.');
  const page = value.page;
  if (
    !Number.isSafeInteger(page.limit) ||
    Number(page.limit) < 1 ||
    Number(page.limit) > 100 ||
    value.items.length > Number(page.limit) ||
    typeof page.has_more !== 'boolean' ||
    (page.has_more
      ? !exactKeys(page, ['limit', 'has_more', 'next_cursor']) ||
        typeof page.next_cursor !== 'string' ||
        !cursorPattern.test(page.next_cursor)
      : !exactKeys(page, ['limit', 'has_more'])) ||
    new Set(value.items.map((row) => row.id)).size !== value.items.length
  )
    throw new Error('Source settings page is invalid.');
  return value as SourceSettingsPage;
};

const parsePreview = (value: unknown): SourceUpdatePreview => {
  if (
    !isRecord(value) ||
    !exactKeys(value, [
      'source',
      'requested_enabled',
      'impact',
      'confirmation_token',
      'expires_at',
    ]) ||
    !isSourceSettingsRecord(value.source) ||
    typeof value.requested_enabled !== 'boolean' ||
    value.impact !== (value.requested_enabled ? 'collection.start' : 'collection.stop') ||
    typeof value.confirmation_token !== 'string' ||
    !proofPattern.test(value.confirmation_token) ||
    typeof value.expires_at !== 'string' ||
    value.expires_at.length < 20 ||
    value.expires_at.length > 30 ||
    !timestampPattern.test(value.expires_at) ||
    !Number.isFinite(Date.parse(value.expires_at))
  )
    throw new Error('Source preview response is invalid.');
  return {
    source: value.source,
    requested_enabled: value.requested_enabled,
    impact: value.requested_enabled ? 'collection.start' : 'collection.stop',
    confirmation_token: value.confirmation_token,
    expires_at: value.expires_at,
  };
};

function sourcePath(source: SourceSettingsRecord) {
  if (!isSourceSettingsRecord(source)) throw new Error('Source settings target is invalid.');
  return `/settings/sources/${source.id}` as const;
}

export function createSourceSettingsApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): SourceSettingsApi {
  return {
    async listSourceSettings(options = {}) {
      if (options.cursor !== undefined && !cursorPattern.test(options.cursor))
        throw new Error('Source settings cursor is invalid.');
      const response = await client.request<unknown>({
        path: '/settings/sources',
        query: { limit: 50, ...(options.cursor === undefined ? {} : { cursor: options.cursor }) },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      const page = parseSourcePage(response.data);
      if (page.page.has_more && page.page.next_cursor === options.cursor)
        throw new Error('Source settings cursor did not advance.');
      return page;
    },
    async previewSourceUpdate(source, enabled, signal) {
      const path = sourcePath(source);
      if (
        typeof enabled !== 'boolean' ||
        source.enabled === enabled ||
        sessionActions === undefined
      )
        throw new Error('Source preview is unavailable.');
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown, SourcePreviewRequest>({
          body: { enabled },
          headers: { 'If-Match': `"${String(source.version)}"` },
          method: 'POST',
          path: `${path}/preview`,
          protection,
          ...(signal === undefined ? {} : { signal }),
        });
        const preview = parsePreview(response.data);
        if (
          preview.source.id !== source.id ||
          preview.source.version !== source.version ||
          preview.requested_enabled !== enabled
        )
          throw new Error('Source preview target is invalid.');
        return preview;
      });
    },
    async updateSource(preview, currentPassword) {
      const checked = parsePreview(preview);
      const path = sourcePath(checked.source);
      if (sessionActions?.runRecentAction === undefined)
        throw new Error('Source update is unavailable.');
      return sessionActions.runRecentAction(
        'source.update',
        currentPassword,
        async (protection) => {
          try {
            const body: SourceUpdateRequest = {
              enabled: checked.requested_enabled,
              confirmation_token: checked.confirmation_token,
            };
            const response = await client.request<unknown, SourceUpdateRequest>({
              body,
              headers: { 'If-Match': `"${String(checked.source.version)}"` },
              method: 'PATCH',
              path,
              protection,
            });
            if (
              !isSourceSettingsRecord(response.data) ||
              response.data.id !== checked.source.id ||
              response.data.enabled !== checked.requested_enabled ||
              response.data.version <= checked.source.version
            )
              throw new SourceSettingsUncertainError();
            return response.data;
          } catch (error) {
            if (error instanceof ApiProblemError) throw error;
            throw new SourceSettingsUncertainError();
          }
        },
      );
    },
  };
}
