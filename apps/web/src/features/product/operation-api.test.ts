import { ApiProblemError, type ApiClient } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';
import {
  createOperationApi,
  OperationUncertainError,
  parseOperationPreview,
  parseOperationReceipt,
  type OperationInput,
} from './operation-api';
import type { SessionActionRunner } from './product-api-types';

const proof = 'A'.repeat(43);
const input: OperationInput = {
  source_id: 'source_cluster',
  resource_id: `r1_${'A'.repeat(43)}`,
  action: 'kubernetes.workload_scale',
  expected_version: 'kv_before',
  parameters: { replicas: 0 },
};
const preview = {
  ...input,
  target_name: 'API',
  effect_code: 'operation.kubernetes_scale',
  risk_code: 'operation.capacity_change',
  current_replicas: 3,
  confirmation_token: proof,
  expires_at: '2099-01-02T03:04:05Z',
};
const receipt = {
  ...input,
  id: 'operation_one',
  state: 'succeeded' as const,
  reason_code: 'operation.succeeded',
  created_at: '2026-01-02T03:04:05Z',
  updated_at: '2026-01-02T03:04:06Z',
  result: {
    previous_version: 'kv_before',
    current_version: 'kv_after',
    effect_code: 'operation.kubernetes_scale_accepted',
  },
};
const runner: SessionActionRunner = {
  runSessionAction: async (action) => action({ kind: 'session', csrfToken: proof }),
  runRecentAction: async (_purpose, _password, action) =>
    action({ kind: 'session', csrfToken: proof, recentAuthToken: proof }),
};
const response = (data: unknown) => ({ data, requestId: 'request_one', status: 200 });

describe('guarded operation transport', () => {
  it('binds preview material, preserves zero scale and sends proofs only in protection/header', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(response({ data: preview }))
      .mockResolvedValueOnce(response({ data: receipt }));
    const purposes: string[] = [];
    const recent: NonNullable<SessionActionRunner['runRecentAction']> = async (
      purpose,
      _password,
      action,
    ) => {
      purposes.push(purpose);
      return action({ kind: 'session', csrfToken: proof, recentAuthToken: proof });
    };
    const api = createOperationApi(
      { request: request as ApiClient['request'] },
      {
        ...runner,
        runRecentAction: recent,
      },
    );
    const checked = await api.previewOperation(input);
    expect(checked.parameters.replicas).toBe(0);
    expect(await api.executeOperation(checked, 'fixture-password')).toEqual(receipt);
    expect(purposes).toEqual(['operation.execute']);
    const call = request.mock.calls[1]?.[0];
    if (call === undefined) throw new Error('Execution request was not captured.');
    expect(call).toMatchObject({
      method: 'POST',
      path: '/operations',
      body: { ...input, confirmation_token: proof },
    });
    expect(/^operation_[A-Za-z0-9]+$/u.test(String(call.headers?.['Idempotency-Key']))).toBe(true);
    expect(call.query).toBeUndefined();
    expect(JSON.stringify(call.body)).not.toContain('fixture-password');
    expect(JSON.stringify(call.body)).not.toContain('Idempotency');
  });
  it('never retries a lost or malformed execution reply', async () => {
    for (const error of [
      new Error('lost'),
      new ApiProblemError({
        status: 503,
        code: 'request.unavailable',
        title: 'Unavailable',
        type: 'urn:stackplore:problem:request.unavailable',
        request_id: 'request_one',
      }),
    ]) {
      const request = vi.fn<ApiClient['request']>().mockRejectedValue(error);
      const api = createOperationApi({ request: request as ApiClient['request'] }, runner);
      await expect(api.executeOperation(preview, 'fixture-password')).rejects.toBeInstanceOf(
        OperationUncertainError,
      );
      expect(request).toHaveBeenCalledTimes(1);
    }
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response({ data: { ...receipt, resource_id: `r1_${'B'.repeat(43)}` } }));
    await expect(
      createOperationApi({ request: request as ApiClient['request'] }, runner).executeOperation(
        preview,
        'fixture-password',
      ),
    ).rejects.toBeInstanceOf(OperationUncertainError);
    expect(request).toHaveBeenCalledTimes(1);
  });
  it('keeps a session-generation change after dispatch uncertain', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response({ data: receipt }));
    const changed: SessionActionRunner = {
      ...runner,
      runRecentAction: async (_purpose, _password, action) => {
        await action({ kind: 'session', csrfToken: proof, recentAuthToken: proof });
        throw new Error('session changed');
      },
    };
    await expect(
      createOperationApi({ request: request as ApiClient['request'] }, changed).executeOperation(
        preview,
        'fixture-password',
      ),
    ).rejects.toBeInstanceOf(OperationUncertainError);
    expect(request).toHaveBeenCalledTimes(1);
  });
  it('rejects foreign targets, unknown fields, forged outcomes and expired confirmations', async () => {
    expect(() =>
      parseOperationPreview({ data: { ...preview, endpoint: 'https://other' } }),
    ).toThrow();
    expect(() => parseOperationReceipt({ data: { ...receipt, state: 'uncertain' } })).toThrow();
    expect(() =>
      parseOperationReceipt({
        data: { ...receipt, result: { ...receipt.result, previous_version: 'kv_other' } },
      }),
    ).toThrow();
    expect(() =>
      parseOperationReceipt({ data: { ...receipt, idempotency_key: 'secret' } }),
    ).toThrow();
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(response({ data: { ...preview, source_id: 'source_other' } }));
    await expect(
      createOperationApi({ request: request as ApiClient['request'] }, runner).previewOperation(
        input,
      ),
    ).rejects.toThrow('target changed');
    request.mockClear();
    await expect(
      createOperationApi({ request: request as ApiClient['request'] }, runner).executeOperation(
        { ...preview, expires_at: '2000-01-02T03:04:05Z' },
        'fixture-password',
      ),
    ).rejects.toThrow('expired');
    expect(request).not.toHaveBeenCalled();
  });
  it('uses bounded read-only recovery with exact receipt/source and advancing cursor', async () => {
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce(response({ data: receipt }))
      .mockResolvedValueOnce(response({ data: [receipt], page: { limit: 25, has_more: false } }));
    const api = createOperationApi({ request: request as ApiClient['request'] }, runner);
    await expect(api.getOperation('operation_one')).resolves.toEqual(receipt);
    await expect(api.listOperations('source_cluster')).resolves.toMatchObject({ data: [receipt] });
    expect(
      request.mock.calls.every(([call]) => call.method === undefined && call.body === undefined),
    ).toBe(true);
    const repeating = vi
      .fn<ApiClient['request']>()
      .mockResolvedValue(
        response({ data: [], page: { limit: 25, has_more: true, next_cursor: 'YQ' } }),
      );
    await expect(
      createOperationApi({ request: repeating as ApiClient['request'] }, runner).listOperations(
        'source_cluster',
        'YQ',
      ),
    ).rejects.toThrow('target changed');
  });
});
