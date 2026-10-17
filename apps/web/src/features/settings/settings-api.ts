import { ApiProblemError, ApiTransportError, type ApiClient } from '@stackplore/api-client';

import type { SessionActionRunner } from '../product/product-api-types';
import { cleanupClasses, type ApplicationSettingsApi, type CleanupClass } from './settings-types';
import {
  isApplicationSettings,
  isApplicationValues,
  isCleanupPreview,
  isCleanupResult,
  isConfigurationExport,
  isSettingsPreview,
  parseSettingsResponse,
  sameValues,
} from './settings-guards';

export type SettingsFailure =
  | 'invalid'
  | 'session'
  | 'denied'
  | 'stale'
  | 'confirmation'
  | 'rate_limited'
  | 'unavailable'
  | 'uncertain';
export class SettingsUncertainError extends Error {
  constructor() {
    super('Settings action outcome is unknown.');
    this.name = 'SettingsUncertainError';
  }
}

export function settingsFailure(error: unknown): SettingsFailure {
  if (error instanceof SettingsUncertainError) return 'uncertain';
  if (error instanceof ApiProblemError) {
    if (error.problem.code.includes('confirmation')) return 'confirmation';
    if (error.problem.status === 401) return 'session';
    if (error.problem.status === 403) return 'denied';
    if (error.problem.status === 409 || error.problem.status === 412) return 'stale';
    if (error.problem.status === 429) return 'rate_limited';
    if (error.problem.status === 400 || error.problem.status === 422) return 'invalid';
    return 'unavailable';
  }
  return 'unavailable';
}

function requireFresh(expiresAt: string) {
  if (Date.now() >= Date.parse(expiresAt)) throw new Error('Settings preview has expired.');
}
const validScope = (scope: readonly CleanupClass[]) =>
  scope.length >= 1 &&
  scope.length <= 4 &&
  scope.every((name) => cleanupClasses.some((allowed) => name === allowed)) &&
  new Set(scope).size === scope.length;
const matchingScope = (left: readonly CleanupClass[], right: readonly CleanupClass[]) =>
  left.length === right.length && left.every((name) => right.includes(name));

export function createApplicationSettingsApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): ApplicationSettingsApi {
  return {
    async getApplicationSettings(signal) {
      const response = await client.request<unknown>({
        path: '/settings/application',
        ...(signal === undefined ? {} : { signal }),
      });
      return parseSettingsResponse(response.data, isApplicationSettings);
    },
    async previewApplicationSettings(settings, requested, signal) {
      if (
        !isApplicationSettings(settings) ||
        !isApplicationValues(requested) ||
        sessionActions === undefined
      )
        throw new Error('Settings preview is unavailable.');
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown, { readonly values: typeof requested }>({
          path: '/settings/application/previews',
          method: 'POST',
          headers: { 'If-Match': `"${String(settings.version)}"` },
          body: { values: requested },
          protection,
          ...(signal === undefined ? {} : { signal }),
        });
        const preview = parseSettingsResponse(response.data, isSettingsPreview);
        if (
          preview.settings.version !== settings.version ||
          !sameValues(preview.settings.values, settings.values) ||
          !sameValues(preview.requested, requested)
        )
          throw new Error('Settings preview does not match its request.');
        return preview;
      });
    },
    async updateApplicationSettings(preview, currentPassword, backupAcknowledged) {
      if (
        !isSettingsPreview(preview) ||
        (preview.impact.backup_required && !backupAcknowledged) ||
        sessionActions?.runRecentAction === undefined
      )
        throw new Error('Settings update is unavailable.');
      requireFresh(preview.expires_at);
      return sessionActions.runRecentAction(
        'settings.update',
        currentPassword,
        async (protection) => {
          try {
            const response = await client.request<
              unknown,
              {
                readonly values: typeof preview.requested;
                readonly confirmation_token: string;
                readonly backup_acknowledged: boolean;
              }
            >({
              path: '/settings/application',
              method: 'PUT',
              headers: { 'If-Match': `"${String(preview.settings.version)}"` },
              protection,
              body: {
                values: preview.requested,
                confirmation_token: preview.confirmation_token,
                backup_acknowledged: backupAcknowledged,
              },
            });
            const updated = parseSettingsResponse(response.data, isApplicationSettings);
            if (
              updated.version <= preview.settings.version ||
              !sameValues(updated.values, preview.requested)
            )
              throw new SettingsUncertainError();
            return updated;
          } catch (error) {
            if (error instanceof ApiProblemError && error.problem.status < 500) throw error;
            throw new SettingsUncertainError();
          }
        },
      );
    },
    async exportConfiguration(currentPassword) {
      if (sessionActions?.runRecentAction === undefined)
        throw new Error('Configuration export is unavailable.');
      return sessionActions.runRecentAction(
        'configuration.export',
        currentPassword,
        async (protection) => {
          const response = await client.request<unknown, Record<string, never>>({
            path: '/settings/export',
            method: 'POST',
            body: {},
            protection,
          });
          return parseSettingsResponse(response.data, isConfigurationExport);
        },
      );
    },
    async previewCleanup(settings, classes, signal) {
      if (!isApplicationSettings(settings) || !validScope(classes) || sessionActions === undefined)
        throw new Error('Cleanup preview is unavailable.');
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown, { readonly classes: typeof classes }>({
          path: '/settings/cleanup/previews',
          method: 'POST',
          headers: { 'If-Match': `"${String(settings.version)}"` },
          body: { classes },
          protection,
          ...(signal === undefined ? {} : { signal }),
        });
        const preview = parseSettingsResponse(response.data, isCleanupPreview);
        if (
          preview.policy_version !== settings.version ||
          !matchingScope(
            classes,
            preview.impact.classes.map((item) => item.name),
          )
        )
          throw new Error('Cleanup preview does not match its scope.');
        return preview;
      });
    },
    async cleanupData(preview, currentPassword, backupAcknowledged) {
      if (
        !isCleanupPreview(preview) ||
        !backupAcknowledged ||
        sessionActions?.runRecentAction === undefined
      )
        throw new Error('Cleanup is unavailable.');
      requireFresh(preview.expires_at);
      return sessionActions.runRecentAction('data.cleanup', currentPassword, async (protection) => {
        try {
          const response = await client.request<
            unknown,
            { readonly confirmation_token: string; readonly backup_acknowledged: true }
          >({
            path: '/settings/cleanup',
            method: 'POST',
            headers: { 'If-Match': `"${String(preview.policy_version)}"` },
            protection,
            body: { confirmation_token: preview.confirmation_token, backup_acknowledged: true },
          });
          const result = parseSettingsResponse(response.data, isCleanupResult);
          if (
            result.policy_version !== preview.policy_version ||
            !matchingScope(
              result.scope,
              preview.impact.classes.map((item) => item.name),
            ) ||
            result.cutoffs.some(
              (cutoff) =>
                !preview.impact.classes.some(
                  (item) => item.name === cutoff.name && item.cutoff === cutoff.cutoff,
                ),
            )
          )
            throw new SettingsUncertainError();
          return result;
        } catch (error) {
          if (error instanceof ApiProblemError && error.problem.status < 500) throw error;
          throw new SettingsUncertainError();
        }
      });
    },
  };
}

export function downloadConfiguration(data: unknown): void {
  const checked = parseSettingsResponse(data, isConfigurationExport);
  const content = JSON.stringify(checked);
  if (new TextEncoder().encode(content).byteLength > 65536)
    throw new ApiTransportError('response_too_large');
  const url = URL.createObjectURL(new Blob([content], { type: 'application/json' }));
  try {
    const link = document.createElement('a');
    link.href = url;
    link.download = 'stackplore-configuration.json';
    link.click();
  } finally {
    URL.revokeObjectURL(url);
  }
}
