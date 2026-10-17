import { ApiTransportError, type ApiClient } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import type { RecentAuthPurpose } from '../auth/auth-api';
import type { SessionActionRunner } from '../product/product-api-types';
import { createApplicationSettingsApi, settingsFailure } from './settings-api';
import {
  isApplicationSettings,
  isCleanupImpact,
  isCleanupPreview,
  isCleanupResult,
  isConfigurationExport,
  isSettingsPreview,
} from './settings-guards';
import {
  cleanupPreviewFixture,
  cleanupResultFixture,
  exportFixture,
  settingsFixture,
  settingsPreviewFixture,
} from './settings-test-fixtures';

function harness(responseData: unknown) {
  const request = vi
    .fn<ApiClient['request']>()
    .mockResolvedValue({ data: responseData, status: 200, requestId: 'request_settings_fixture' });
  const recent: RecentAuthPurpose[] = [];
  const protection = {
    kind: 'session' as const,
    csrfToken: 'A'.repeat(43),
    recentAuthToken: 'A'.repeat(43),
  };
  const actions: SessionActionRunner = {
    runSessionAction: async (action) => action(protection),
    runRecentAction: async (purpose, _password, action) => {
      recent.push(purpose);
      return action(protection);
    },
  };
  return {
    api: createApplicationSettingsApi({ request: request as ApiClient['request'] }, actions),
    request,
    recent,
  };
}

describe('closed application settings boundaries', () => {
  it('accepts the exact canonical safe models', () => {
    expect(isApplicationSettings(settingsFixture)).toBe(true);
    expect(isSettingsPreview(settingsPreviewFixture)).toBe(true);
    expect(isCleanupPreview(cleanupPreviewFixture)).toBe(true);
    expect(isCleanupResult(cleanupResultFixture)).toBe(true);
    expect(isConfigurationExport(exportFixture)).toBe(true);
  });
  it.each([
    { ...settingsFixture, secret: 'invented-disallowed-field' },
    { ...settingsFixture, values: { ...settingsFixture.values, log_line_limit: 201 } },
    { ...settingsFixture, values: { ...settingsFixture.values, metric_retention_hours: 0 } },
    { ...settingsFixture, values: { ...settingsFixture.values, health_retention_hours: 8761 } },
    {
      ...settingsFixture,
      values: { ...settingsFixture.values, default_alert_pending_seconds: 0.5 },
    },
    { ...settingsFixture, audit_policy: { ...settingsFixture.audit_policy, retention_days: 89 } },
    { ...settingsFixture, audit_policy: { ...settingsFixture.audit_policy, quota_rows: 10000001 } },
    { ...settingsFixture, updated_at: '2026-02-30T08:00:00Z' },
    { ...settingsFixture, updated_at: '2026-08-16T08:00:00.1234567Z' },
  ])('rejects invalid/extra settings safely', (value) => {
    expect(isApplicationSettings(value)).toBe(false);
  });
  it('binds retention impact to exactly the decreased classes and truthful capped counts', () => {
    expect(
      isSettingsPreview({
        ...settingsPreviewFixture,
        impact: { ...settingsPreviewFixture.impact, classes: [] },
      }),
    ).toBe(false);
    expect(
      isSettingsPreview({
        ...settingsPreviewFixture,
        impact: { ...settingsPreviewFixture.impact, expiry_policy_changed: false },
      }),
    ).toBe(false);
    expect(
      isCleanupImpact({
        ...settingsPreviewFixture.impact,
        classes: [{ ...settingsPreviewFixture.impact.classes[0], eligible_rows_capped: false }],
      }),
    ).toBe(false);
    expect(
      isCleanupImpact({
        ...settingsPreviewFixture.impact,
        classes: [{ ...settingsPreviewFixture.impact.classes[0], eligible_rows: 10000 }],
      }),
    ).toBe(false);
    expect(
      isCleanupPreview({
        ...cleanupPreviewFixture,
        impact: { ...cleanupPreviewFixture.impact, expiry_policy_changed: true },
      }),
    ).toBe(false);
    expect(
      isCleanupPreview({
        ...cleanupPreviewFixture,
        impact: { ...cleanupPreviewFixture.impact, backup_required: false },
      }),
    ).toBe(false);
    expect(
      isCleanupResult({ ...cleanupResultFixture, scope: ['metric_samples', 'audit_entries'] }),
    ).toBe(false);
    expect(
      isCleanupResult({
        ...cleanupResultFixture,
        cutoffs: [{ name: 'health_history', cutoff: '2026-08-09T08:00:00Z' }],
      }),
    ).toBe(false);
  });
  it.each([
    { ...exportFixture, raw_environment: {} },
    { ...exportFixture, sources: [{ ...exportFixture.sources[0], host: 'unmasked-host' }] },
    {
      ...exportFixture,
      sources: [{ ...exportFixture.sources[0], credential_reference: 'unmasked-reference' }],
    },
    {
      ...exportFixture,
      notification_integrations: [
        { ...exportFixture.notification_integrations[0], target: 'unmasked-target' },
      ],
    },
    { ...exportFixture, sources: Array.from({ length: 101 }, () => exportFixture.sources[0]) },
  ])('rejects export material outside the exact masked allowlist', (value) => {
    expect(isConfigurationExport(value)).toBe(false);
  });
});

describe('settings transport and one-use workflows', () => {
  it('binds preview and update to exact values, version and recent purpose without a password in operation payload', async () => {
    const setup = harness(settingsPreviewFixture);
    const preview = await setup.api.previewApplicationSettings(
      settingsFixture,
      settingsPreviewFixture.requested,
    );
    expect(setup.request.mock.calls[0]?.[0]).toMatchObject({
      method: 'POST',
      path: '/settings/application/previews',
      headers: { 'If-Match': '"1"' },
      body: { values: preview.requested },
    });
    setup.request.mockResolvedValueOnce({
      data: { ...settingsFixture, values: preview.requested, version: 2 },
      status: 200,
      requestId: 'request_settings_update',
    });
    await setup.api.updateApplicationSettings(preview, 'fixture-password-123', true);
    expect(setup.recent).toEqual(['settings.update']);
    expect(setup.request.mock.calls[1]?.[0]).toMatchObject({
      method: 'PUT',
      path: '/settings/application',
      headers: { 'If-Match': '"1"' },
      body: {
        values: preview.requested,
        confirmation_token: preview.confirmation_token,
        backup_acknowledged: true,
      },
    });
    expect(JSON.stringify(setup.request.mock.calls)).not.toContain('fixture-password-123');
  });
  it('does not send an expired or unacknowledged destructive preview', async () => {
    const setup = harness(settingsFixture);
    await expect(
      setup.api.updateApplicationSettings(settingsPreviewFixture, 'fixture-password-123', false),
    ).rejects.toThrow();
    await expect(
      setup.api.cleanupData(
        { ...cleanupPreviewFixture, expires_at: '2000-08-16T08:00:00Z' },
        'fixture-password-123',
        true,
      ),
    ).rejects.toThrow();
    expect(setup.request).not.toHaveBeenCalled();
    expect(setup.recent).toEqual([]);
  });
  it('binds cleanup result scope and cutoffs, with a separate recent purpose', async () => {
    const setup = harness(cleanupResultFixture);
    await setup.api.cleanupData(cleanupPreviewFixture, 'fixture-password-123', true);
    expect(setup.recent).toEqual(['data.cleanup']);
    expect(setup.request.mock.calls[0]?.[0]).toMatchObject({
      path: '/settings/cleanup',
      headers: { 'If-Match': '"1"' },
      body: {
        confirmation_token: cleanupPreviewFixture.confirmation_token,
        backup_acknowledged: true,
      },
    });
    setup.request.mockResolvedValueOnce({
      data: {
        ...cleanupResultFixture,
        cutoffs: [{ name: 'metric_samples', cutoff: '2026-08-08T08:00:00Z' }],
      },
      status: 200,
      requestId: 'request_wrong_cutoff',
    });
    const failure = await setup.api
      .cleanupData(cleanupPreviewFixture, 'fixture-password-123', true)
      .catch((error: unknown) => error);
    expect(settingsFailure(failure)).toBe('uncertain');
  });
  it('reports a lost write acknowledgement as uncertain and never retries automatically', async () => {
    const setup = harness(settingsFixture);
    setup.request.mockRejectedValueOnce(new ApiTransportError('network_error'));
    const failure = await setup.api
      .updateApplicationSettings(settingsPreviewFixture, 'fixture-password-123', true)
      .catch((error: unknown) => error);
    expect(settingsFailure(failure)).toBe('uncertain');
    expect(setup.request).toHaveBeenCalledTimes(1);
  });
  it('exports only through a recent configuration.export action', async () => {
    const setup = harness(exportFixture);
    expect(await setup.api.exportConfiguration('fixture-password-123')).toEqual(exportFixture);
    expect(setup.recent).toEqual(['configuration.export']);
    expect(setup.request.mock.calls[0]?.[0]).toMatchObject({
      path: '/settings/export',
      method: 'POST',
      body: {},
    });
  });
});
