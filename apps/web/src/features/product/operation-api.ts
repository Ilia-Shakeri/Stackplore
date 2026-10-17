import {
  ApiProblemError,
  type ApiClient,
  type OperationPage,
  type OperationPreview,
  type OperationReceipt,
} from '@stackplore/api-client';
import type { SessionActionRunner } from './product-api-types';
import {
  cursorPattern,
  hasExactKeys,
  isDisplayText,
  isRecord,
  isTimestamp,
  namespacedCodePattern,
  resourceIdPattern,
  tokenPattern,
} from './product-api-common';

export type OperationPreviewData = OperationPreview['data'];
export type OperationInput = Pick<
  OperationPreviewData,
  'source_id' | 'resource_id' | 'action' | 'expected_version' | 'parameters'
>;
export interface OperationApi {
  previewOperation(input: OperationInput, signal?: AbortSignal): Promise<OperationPreviewData>;
  executeOperation(preview: OperationPreviewData, password: string): Promise<OperationReceipt>;
  getOperation(id: string, signal?: AbortSignal): Promise<OperationReceipt>;
  listOperations(sourceId: string, cursor?: string, signal?: AbortSignal): Promise<OperationPage>;
}
export class OperationUncertainError extends Error {
  constructor() {
    super('Operation outcome is unknown. Check durable history before another action.');
    this.name = 'OperationUncertainError';
  }
}
const actions = [
  'docker.container_restart',
  'kubernetes.workload_restart',
  'kubernetes.workload_scale',
];
const proofPattern = /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const integer = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 && value <= 10000;
const token = (value: unknown): value is string =>
  typeof value === 'string' && tokenPattern.test(value);
const code = (value: unknown): value is string =>
  typeof value === 'string' && value.length <= 96 && namespacedCodePattern.test(value);
const inputKeys = ['source_id', 'resource_id', 'action', 'expected_version', 'parameters'];
const operationTimestamp = (value: unknown): value is string =>
  isTimestamp(value) && value.length <= 27;
function validMaterial(value: Record<string, unknown>): boolean {
  if (
    !token(value.source_id) ||
    typeof value.resource_id !== 'string' ||
    !resourceIdPattern.test(value.resource_id) ||
    !actions.includes(String(value.action)) ||
    !token(value.expected_version) ||
    !isRecord(value.parameters)
  )
    return false;
  return value.action === 'kubernetes.workload_scale'
    ? hasExactKeys(value.parameters, ['replicas']) && integer(value.parameters.replicas)
    : hasExactKeys(value.parameters, []);
}
function material(input: OperationInput): OperationInput {
  if (!isRecord(input) || !hasExactKeys(input, inputKeys) || !validMaterial(input))
    throw new Error('Operation target is invalid.');
  return {
    source_id: input.source_id,
    resource_id: input.resource_id,
    action: input.action,
    expected_version: input.expected_version,
    parameters: { ...input.parameters },
  };
}
const sameMaterial = (left: OperationInput, right: OperationInput) =>
  left.source_id === right.source_id &&
  left.resource_id === right.resource_id &&
  left.action === right.action &&
  left.expected_version === right.expected_version &&
  left.parameters.replicas === right.parameters.replicas;
export function parseOperationPreview(value: unknown): OperationPreviewData {
  if (!isRecord(value) || !hasExactKeys(value, ['data']) || !isRecord(value.data))
    throw new Error('Operation preview is invalid.');
  const row = value.data;
  if (
    !hasExactKeys(
      row,
      [...inputKeys, 'target_name', 'effect_code', 'risk_code', 'confirmation_token', 'expires_at'],
      ['current_replicas'],
    ) ||
    !validMaterial(row) ||
    !isDisplayText(row.target_name) ||
    !code(row.effect_code) ||
    !code(row.risk_code) ||
    typeof row.confirmation_token !== 'string' ||
    !proofPattern.test(row.confirmation_token) ||
    !operationTimestamp(row.expires_at) ||
    (row.action === 'kubernetes.workload_scale'
      ? !integer(row.current_replicas)
      : row.current_replicas !== undefined)
  )
    throw new Error('Operation preview is invalid.');
  return row as OperationPreviewData;
}
export function parseOperationReceipt(value: unknown): OperationReceipt {
  if (!isRecord(value) || !hasExactKeys(value, ['data']) || !isRecord(value.data))
    throw new Error('Operation receipt is invalid.');
  const row = value.data;
  if (
    !hasExactKeys(
      row,
      [...inputKeys, 'id', 'state', 'created_at', 'updated_at'],
      ['reason_code', 'result'],
    ) ||
    !validMaterial(row) ||
    !token(row.id) ||
    !['in_progress', 'succeeded', 'failed', 'uncertain'].includes(String(row.state)) ||
    !operationTimestamp(row.created_at) ||
    !operationTimestamp(row.updated_at) ||
    Date.parse(row.updated_at) < Date.parse(row.created_at)
  )
    throw new Error('Operation receipt is invalid.');
  if (row.state === 'in_progress') {
    if (row.reason_code !== undefined || row.result !== undefined)
      throw new Error('Pending operation has a result.');
  } else if (!code(row.reason_code)) throw new Error('Terminal operation has no reason.');
  if (row.state === 'succeeded') {
    if (
      row.reason_code !== 'operation.succeeded' ||
      !isRecord(row.result) ||
      !hasExactKeys(row.result, ['previous_version', 'current_version', 'effect_code']) ||
      row.result.previous_version !== row.expected_version ||
      !token(row.result.current_version) ||
      !code(row.result.effect_code)
    )
      throw new Error('Operation result is invalid.');
  } else if (row.result !== undefined) throw new Error('Unconfirmed operation has a result.');
  return row as OperationReceipt;
}
function parsePage(value: unknown): OperationPage {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['data', 'page']) ||
    !Array.isArray(value.data) ||
    value.data.length > 100 ||
    !isRecord(value.page)
  )
    throw new Error('Operation page is invalid.');
  const page = value.page;
  if (
    !Number.isSafeInteger(page.limit) ||
    Number(page.limit) < 1 ||
    Number(page.limit) > 100 ||
    value.data.length > Number(page.limit) ||
    typeof page.has_more !== 'boolean' ||
    (page.has_more
      ? !hasExactKeys(page, ['limit', 'has_more', 'next_cursor']) ||
        typeof page.next_cursor !== 'string' ||
        !cursorPattern.test(page.next_cursor)
      : !hasExactKeys(page, ['limit', 'has_more']))
  )
    throw new Error('Operation page is invalid.');
  const rows = (value.data as unknown[]).map((row) => parseOperationReceipt({ data: row }));
  if (new Set(rows.map((row) => row.id)).size !== rows.length)
    throw new Error('Duplicate operation receipt.');
  return { data: rows, page: page as OperationPage['page'] };
}
export function createOperationApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): OperationApi {
  return {
    async previewOperation(input, signal) {
      const body = material(input);
      if (sessionActions === undefined) throw new Error('Operation preview is unavailable.');
      return sessionActions.runSessionAction(async (protection) => {
        const response = await client.request<unknown, OperationInput>({
          method: 'POST',
          path: '/operations/previews',
          body,
          protection,
          ...(signal === undefined ? {} : { signal }),
        });
        const preview = parseOperationPreview(response.data);
        if (!sameMaterial(body, preview)) throw new Error('Operation preview target changed.');
        return preview;
      });
    },
    async executeOperation(preview, password) {
      const checked = parseOperationPreview({ data: preview });
      if (
        Date.parse(checked.expires_at) <= Date.now() ||
        sessionActions?.runRecentAction === undefined
      )
        throw new Error('Operation confirmation expired.');
      const idempotencyKey = `operation_${crypto.randomUUID().replaceAll('-', '')}`;
      const dispatchState = { dispatched: false };
      try {
        return await sessionActions.runRecentAction(
          'operation.execute',
          password,
          async (protection) => {
            try {
              const body = {
                ...material({
                  source_id: checked.source_id,
                  resource_id: checked.resource_id,
                  action: checked.action,
                  expected_version: checked.expected_version,
                  parameters: checked.parameters,
                }),
                confirmation_token: checked.confirmation_token,
              };
              dispatchState.dispatched = true;
              const response = await client.request<unknown, typeof body>({
                method: 'POST',
                path: '/operations',
                body,
                headers: { 'Idempotency-Key': idempotencyKey },
                protection,
              });
              const receipt = parseOperationReceipt(response.data);
              if (!sameMaterial(receipt, checked)) throw new OperationUncertainError();
              return receipt;
            } catch (error) {
              if (error instanceof ApiProblemError && error.problem.status < 500) throw error;
              throw new OperationUncertainError();
            }
          },
        );
      } catch (error) {
        if (error instanceof ApiProblemError && error.problem.status < 500) throw error;
        if (dispatchState.dispatched) throw new OperationUncertainError();
        throw error;
      }
    },
    async getOperation(id, signal) {
      if (!token(id)) throw new Error('Operation identifier is invalid.');
      const response = await client.request<unknown>({
        path: `/operations/${id}`,
        ...(signal === undefined ? {} : { signal }),
      });
      const row = parseOperationReceipt(response.data);
      if (row.id !== id) throw new Error('Operation receipt changed.');
      return row;
    },
    async listOperations(sourceId, cursor, signal) {
      if (!token(sourceId) || (cursor !== undefined && !cursorPattern.test(cursor)))
        throw new Error('Operation page input is invalid.');
      const response = await client.request<unknown>({
        path: '/operations',
        query: { source_id: sourceId, limit: 25, ...(cursor === undefined ? {} : { cursor }) },
        ...(signal === undefined ? {} : { signal }),
      });
      const page = parsePage(response.data);
      if (
        page.page.limit !== 25 ||
        page.data.some((row) => row.source_id !== sourceId) ||
        (page.page.has_more && page.page.next_cursor === cursor)
      )
        throw new Error('Operation page target changed.');
      return page;
    },
  };
}
