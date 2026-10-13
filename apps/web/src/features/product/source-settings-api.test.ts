import {
  ApiProblemError,
  ApiTransportError,
  type ApiClient,
  type SourceSettingsRecord,
  type SourceUpdatePreview,
} from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import {
  classifySourceSettingsFailure,
  createSourceSettingsApi,
  SourceSettingsUncertainError,
} from './source-settings-api';
import type { SessionActionRunner } from './product-api';

const proof = 'A'.repeat(43);
const source: SourceSettingsRecord = {
  id: 'source_docker',
  provider: 'docker',
  name: 'Local Docker',
  synthetic: false,
  enabled: false,
  version: 1,
  profile_available: true,
};
const preview: SourceUpdatePreview = {
  source,
  requested_enabled: true,
  impact: 'collection.start',
  confirmation_token: proof,
  expires_at: '2099-01-02T03:04:05Z',
};
const response = (data: unknown) => ({ data, status: 200, requestId: 'request_source_test' });
const rejected = (code: string, status: number) =>
  new ApiProblemError({
    code,
    status,
    type: `urn:stackplore:problem:${code}`,
    title: 'Request rejected',
    request_id: 'request_source_test',
  });
const runner: SessionActionRunner = {
  runSessionAction: async (action) => await action({ kind: 'session', csrfToken: proof }),
  runRecentAction: async (_purpose, _password, action) =>
    await action({ kind: 'session', csrfToken: proof, recentAuthToken: `${'B'.repeat(42)}A` }),
};

describe('source settings transport', () => {
  it('owns bounded source settings pages and their cursor', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response({ items: [source], page: { limit: 50, has_more: false } }));
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] }, runner);
    await expect(api.listSourceSettings({ cursor: 'source_next' })).resolves.toMatchObject({
      items: [source],
    });
    expect(request).toHaveBeenCalledWith({
      path: '/settings/sources',
      query: { limit: 50, cursor: 'source_next' },
    });
  });

  it.each([
    { ...source, url: 'https://example.invalid' },
    { ...source, name: 'Unsafe\u202eName' },
    { ...source, enabled: 'true' },
    { ...source, version: Number.MAX_SAFE_INTEGER + 1 },
    { ...source, profile_available: 'yes' },
  ])('rejects drifted source records without echoing fields', async (item) => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response({ items: [item], page: { limit: 50, has_more: false } }));
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] });
    await expect(api.listSourceSettings()).rejects.toThrow('Source settings response is invalid.');
  });

  it.each([
    { items: [source, source], page: { limit: 50, has_more: false } },
    { items: [source], page: { limit: 50, has_more: true } },
    { items: Array.from({ length: 101 }, () => source), page: { limit: 100, has_more: false } },
  ])('rejects duplicate, incomplete or oversized pages', async (page) => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(page));
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] });
    await expect(api.listSourceSettings()).rejects.toThrow();
  });

  it('previews only the exact source version with session protection', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(preview));
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] }, runner);
    await expect(api.previewSourceUpdate(source, true)).resolves.toEqual(preview);
    expect(request).toHaveBeenCalledWith({
      path: '/settings/sources/source_docker/preview',
      method: 'POST',
      body: { enabled: true },
      headers: { 'If-Match': '"1"' },
      protection: { kind: 'session', csrfToken: proof },
    });
  });

  it('rejects a repeated forward cursor', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        items: [source],
        page: { limit: 50, has_more: true, next_cursor: 'source_next' },
      }),
    );
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] });
    await expect(api.listSourceSettings({ cursor: 'source_next' })).rejects.toThrow(
      'Source settings cursor did not advance.',
    );
  });

  it.each([1025, 4096])(
    'accepts bounded profile cursor length %s in both directions',
    async (length) => {
      const cursor = 'A'.repeat(length);
      const request = vi
        .fn<ApiClient['request']>()
        .mockResolvedValueOnce(
          response({ items: [source], page: { limit: 50, has_more: true, next_cursor: cursor } }),
        )
        .mockResolvedValueOnce(response({ items: [source], page: { limit: 50, has_more: false } }));
      const api = createSourceSettingsApi({ request: request as ApiClient['request'] });
      const first = await api.listSourceSettings();
      if (!first.page.has_more) throw new Error('Expected another profile page.');
      expect(first.page.next_cursor).toBe(cursor);
      await api.listSourceSettings({ cursor });
      expect(request.mock.calls[1]?.[0].query).toEqual({ limit: 50, cursor });
    },
  );

  it('rejects oversized profile cursor input before transport and output after transport', async () => {
    const cursor = 'A'.repeat(4097);
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(
        response({ items: [source], page: { limit: 50, has_more: true, next_cursor: cursor } }),
      );
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] });
    await expect(api.listSourceSettings({ cursor })).rejects.toThrow(
      'Source settings cursor is invalid.',
    );
    expect(request).not.toHaveBeenCalled();
    await expect(api.listSourceSettings()).rejects.toThrow('Source settings page is invalid.');
  });

  it.each([
    { ...preview, source: { ...source, id: 'source_other' } },
    { ...preview, source: { ...source, version: 2 } },
    { ...preview, requested_enabled: false, impact: 'collection.stop' },
    { ...preview, impact: 'collection.stop' },
    { ...preview, confirmation_token: 'invalid' },
  ])('rejects mismatched or malformed preview binding', async (data) => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(data));
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] }, runner);
    await expect(api.previewSourceUpdate(source, true)).rejects.toThrow();
  });

  it('obtains one source-purpose grant for each patch and seals target, version and confirmation', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(response({ ...source, enabled: true, version: 2 }))
      .mockRejectedValueOnce(rejected('source.confirmation_rejected', 409));
    const purposes: string[] = [];
    const guarded: SessionActionRunner = {
      ...runner,
      runRecentAction: async (purpose, _password, action) => {
        purposes.push(purpose);
        return await action({
          kind: 'session',
          csrfToken: proof,
          recentAuthToken: `${(purposes.length === 1 ? 'B' : 'C').repeat(42)}A`,
        });
      },
    };
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] }, guarded);
    await expect(api.updateSource(preview, 'invented-password')).resolves.toMatchObject({
      enabled: true,
      version: 2,
    });
    await expect(api.updateSource(preview, 'invented-password')).rejects.toMatchObject({
      problem: { code: 'source.confirmation_rejected' },
    });
    expect(purposes).toEqual(['source.update', 'source.update']);
    expect(request.mock.calls[0]?.[0]).toEqual({
      path: '/settings/sources/source_docker',
      method: 'PATCH',
      body: { enabled: true, confirmation_token: proof },
      headers: { 'If-Match': '"1"' },
      protection: { kind: 'session', csrfToken: proof, recentAuthToken: `${'B'.repeat(42)}A` },
    });
    expect(request.mock.calls[1]?.[0].protection).toMatchObject({
      recentAuthToken: `${'C'.repeat(42)}A`,
    });
  });

  it('does not patch after a recent-auth denial', async () => {
    const request = vi.fn<ApiClient['request']>();
    const api = createSourceSettingsApi(
      { request: request as ApiClient['request'] },
      { ...runner, runRecentAction: () => Promise.reject(rejected('recent_auth_required', 403)) },
    );
    await expect(api.updateSource(preview, 'invented-password')).rejects.toBeInstanceOf(
      ApiProblemError,
    );
    expect(request).not.toHaveBeenCalled();
  });

  it('marks a lost patch reply uncertain without retrying', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockRejectedValue(new ApiTransportError('network_error'));
    const api = createSourceSettingsApi({ request: request as ApiClient['request'] }, runner);
    await expect(api.updateSource(preview, 'invented-password')).rejects.toBeInstanceOf(
      SourceSettingsUncertainError,
    );
    expect(request).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['source.version_conflict', 409, 'stale'],
    ['source.state_unchanged', 409, 'stale'],
    ['source.confirmation_rejected', 409, 'confirmation'],
    ['source.profile_unavailable', 422, 'profile'],
    ['forbidden', 403, 'denied'],
  ] as const)('classifies safe problem %s', (code, status, kind) => {
    expect(classifySourceSettingsFailure(rejected(code, status))).toBe(kind);
  });
});
