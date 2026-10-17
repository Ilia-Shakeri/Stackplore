import type { OperationPage, OperationReceipt, ProductResource } from '@stackplore/api-client';
import { TechnicalText } from '@stackplore/ui';
import { useEffect, useId, useRef, useState, type SyntheticEvent } from 'react';
import { useLocale } from '../../i18n';
import { useOptionalAuthSnapshot } from '../auth/auth-context';
import { isValidPasswordText } from '../auth/auth-form';
import { useProductApi } from './product-context';
import {
  OperationUncertainError,
  type OperationInput,
  type OperationPreviewData,
} from './operation-api';
import './resource-operations.css';

export function ResourceOperationsPanel({ resource }: { readonly resource: ProductResource }) {
  const auth = useOptionalAuthSnapshot();
  if (
    auth?.status !== 'authenticated' ||
    auth.user.role !== 'administrator' ||
    resource.synthetic ||
    resource.operation_version === undefined ||
    !(
      (resource.provider === 'docker' && resource.kind === 'docker.container') ||
      (resource.provider === 'kubernetes' &&
        ['kubernetes.deployment', 'kubernetes.statefulset', 'kubernetes.daemonset'].includes(
          resource.kind,
        ))
    )
  )
    return null;
  return (
    <GuardedActions
      key={`${resource.source_id}:${resource.id}:${resource.operation_version}`}
      resource={resource}
    />
  );
}

function GuardedActions({ resource }: { readonly resource: ProductResource }) {
  const api = useProductApi();
  const { t, formatDate } = useLocale();
  const title = useId();
  const [action, setAction] = useState<OperationInput['action']>(
    resource.provider === 'docker' ? 'docker.container_restart' : 'kubernetes.workload_restart',
  );
  const [replicas, setReplicas] = useState('1');
  const [preview, setPreview] = useState<OperationPreviewData | null>(null);
  const [password, setPassword] = useState('');
  const [accepted, setAccepted] = useState(false);
  const [busy, setBusy] = useState<'preview' | 'execute' | null>(null);
  const [message, setMessage] = useState<'unknown' | 'failedRequest' | 'expired' | null>(null);
  const [history, setHistory] = useState<(OperationPage & { readonly capped: boolean }) | null>(
    null,
  );
  const [historyBusy, setHistoryBusy] = useState(false);
  const [historyError, setHistoryError] = useState(false);
  const [receipt, setReceipt] = useState<OperationReceipt | null>(null);
  const mounted = useRef(true);
  const historyController = useRef<AbortController | null>(null);
  const previewController = useRef<AbortController | null>(null);
  const previewHeading = useRef<HTMLHeadingElement | null>(null);
  const previewButton = useRef<HTMLButtonElement | null>(null);
  const previousPreview = useRef(false);
  const executionLock = useRef(false);
  const canScale =
    resource.kind === 'kubernetes.deployment' || resource.kind === 'kubernetes.statefulset';

  async function loadHistory(append = false) {
    if (historyBusy) return;
    const cursor = append && history?.page.has_more ? history.page.next_cursor : undefined;
    const controller = new AbortController();
    historyController.current?.abort();
    historyController.current = controller;
    setHistoryBusy(true);
    setHistoryError(false);
    try {
      const page = await api.listOperations(resource.source_id, cursor, controller.signal);
      if (!mounted.current || controller.signal.aborted) return;
      setHistory((previous) => {
        const rows = append ? [...(previous?.data ?? []), ...page.data] : page.data;
        const unique = [...new Map(rows.map((row) => [row.id, row])).values()];
        return {
          ...page,
          data: unique.slice(0, 1000),
          capped: unique.length > 1000 || (unique.length === 1000 && page.page.has_more),
        };
      });
    } catch {
      if (mounted.current && !controller.signal.aborted) setHistoryError(true);
    } finally {
      if (mounted.current && !controller.signal.aborted) setHistoryBusy(false);
    }
  }
  useEffect(() => {
    mounted.current = true;
    void loadHistory();
    return () => {
      mounted.current = false;
      historyController.current?.abort();
      previewController.current?.abort();
    };
  }, [api, resource.source_id]);
  useEffect(() => {
    if (preview === null) return;
    const timer = window.setTimeout(
      () => {
        setPreview(null);
        setPassword('');
        setAccepted(false);
        setMessage('expired');
      },
      Math.max(0, Math.min(120000, Date.parse(preview.expires_at) - Date.now())),
    );
    return () => {
      window.clearTimeout(timer);
    };
  }, [preview]);
  useEffect(() => {
    if (preview !== null) {
      previewHeading.current?.focus();
      previousPreview.current = true;
    } else if (previousPreview.current && busy !== 'execute') {
      previewButton.current?.focus();
      previousPreview.current = false;
    }
  }, [preview, busy]);
  useEffect(() => {
    if (receipt?.state !== 'in_progress') return;
    const controller = new AbortController();
    let timer: number | undefined;
    let reads = 0;
    const read = async () => {
      try {
        const row = await api.getOperation(receipt.id, controller.signal);
        if (controller.signal.aborted) return;
        setReceipt(row);
        if (row.state === 'in_progress' && ++reads < 10)
          timer = window.setTimeout(() => {
            void read();
          }, 2000);
        else void loadHistory();
      } catch {
        if (!controller.signal.aborted) setHistoryError(true);
      }
    };
    timer = window.setTimeout(() => {
      void read();
    }, 2000);
    return () => {
      controller.abort();
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [api, receipt?.id, receipt?.state]);

  async function check(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy !== null) return;
    setBusy('preview');
    setMessage(null);
    setAccepted(false);
    setPassword('');
    const controller = new AbortController();
    previewController.current = controller;
    try {
      const result = await api.previewOperation(
        {
          source_id: resource.source_id,
          resource_id: resource.id,
          expected_version: resource.operation_version ?? '',
          action,
          parameters: action === 'kubernetes.workload_scale' ? { replicas: Number(replicas) } : {},
        },
        controller.signal,
      );
      if (mounted.current && !controller.signal.aborted) setPreview(result);
    } catch {
      if (mounted.current && !controller.signal.aborted) setMessage('failedRequest');
    } finally {
      if (mounted.current && !controller.signal.aborted) setBusy(null);
    }
  }
  async function execute(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      preview === null ||
      !accepted ||
      !isValidPasswordText(password) ||
      busy !== null ||
      executionLock.current
    )
      return;
    executionLock.current = true;
    const checked = preview,
      currentPassword = password;
    setPassword('');
    setBusy('execute');
    setMessage(null);
    setPreview(null);
    setAccepted(false);
    try {
      const row = await api.executeOperation(checked, currentPassword);
      if (mounted.current) setReceipt(row);
    } catch (error) {
      if (mounted.current)
        setMessage(error instanceof OperationUncertainError ? 'unknown' : 'failedRequest');
    } finally {
      executionLock.current = false;
      if (mounted.current) {
        setBusy(null);
        void loadHistory();
      }
    }
  }
  return (
    <section className="resource-operations" aria-labelledby={title}>
      <h2 id={title}>{t('operation.title')}</h2>
      <p>{t('operation.description')}</p>
      {preview === null ? (
        <form
          onSubmit={(event) => {
            void check(event);
          }}
        >
          <label>
            {t('operation.action')}
            <select
              value={action}
              disabled={busy !== null}
              onChange={(event) => {
                setAction(
                  event.target.value === 'kubernetes.workload_scale'
                    ? 'kubernetes.workload_scale'
                    : resource.provider === 'docker'
                      ? 'docker.container_restart'
                      : 'kubernetes.workload_restart',
                );
              }}
            >
              <option
                value={
                  resource.provider === 'docker'
                    ? 'docker.container_restart'
                    : 'kubernetes.workload_restart'
                }
              >
                {t('operation.restart')}
              </option>
              {canScale ? (
                <option value="kubernetes.workload_scale">{t('operation.scale')}</option>
              ) : null}
            </select>
          </label>
          {action === 'kubernetes.workload_scale' ? (
            <label>
              {t('operation.replicas')}
              <input
                dir="ltr"
                type="number"
                min="0"
                max="10000"
                step="1"
                required
                value={replicas}
                disabled={busy !== null}
                onChange={(event) => {
                  setReplicas(event.target.value);
                }}
              />
            </label>
          ) : null}
          <button ref={previewButton} type="submit" disabled={busy !== null}>
            {t(
              busy === 'execute'
                ? 'operation.sending'
                : busy === 'preview'
                  ? 'operation.busy'
                  : 'operation.preview',
            )}
          </button>
        </form>
      ) : (
        <form
          onSubmit={(event) => {
            void execute(event);
          }}
          className="resource-operations__preview"
        >
          <h3 ref={previewHeading} tabIndex={-1}>
            {t('operation.previewTitle')}
          </h3>
          <dl>
            <dt>{t('operation.target')}</dt>
            <dd>
              <TechnicalText>{preview.target_name}</TechnicalText>
              <br />
              <TechnicalText>{preview.resource_id}</TechnicalText>
            </dd>
            <dt>{t('operation.version')}</dt>
            <dd>
              <TechnicalText>{preview.expected_version}</TechnicalText>
            </dd>
            {preview.current_replicas !== undefined ? (
              <>
                <dt>{t('operation.capacity')}</dt>
                <dd>
                  <TechnicalText>{`${String(preview.current_replicas)} → ${String(preview.parameters.replicas ?? '')}`}</TechnicalText>
                </dd>
              </>
            ) : null}
          </dl>
          <p>
            {t(
              preview.action === 'kubernetes.workload_scale'
                ? 'operation.riskScale'
                : 'operation.riskRestart',
            )}
          </p>
          <label className="resource-operations__ack">
            <input
              type="checkbox"
              checked={accepted}
              onChange={(event) => {
                setAccepted(event.target.checked);
              }}
              required
            />
            {t('operation.acknowledge')}
          </label>
          <label>
            {t('operation.password')}
            <input
              type="password"
              dir="ltr"
              value={password}
              required
              autoComplete="current-password"
              maxLength={128}
              onChange={(event) => {
                setPassword(event.target.value);
              }}
            />
          </label>
          <div>
            <button type="submit" disabled={!accepted || !isValidPasswordText(password)}>
              {t('operation.confirm')}
            </button>
            <button
              type="button"
              onClick={() => {
                setPreview(null);
                setPassword('');
                setAccepted(false);
              }}
            >
              {t('operation.cancel')}
            </button>
          </div>
        </form>
      )}
      <div role="status" aria-live="polite">
        {message === null ? null : t(`operation.${message}`)}
        {receipt === null ? null : (
          <p>
            {t(`operation.${receipt.state}`)} <TechnicalText>{receipt.id}</TechnicalText>
          </p>
        )}
      </div>
      <h3>{t('operation.history')}</h3>
      <p>{t('operation.historyDescription')}</p>
      <button
        type="button"
        disabled={historyBusy}
        onClick={() => {
          void loadHistory();
        }}
      >
        {t('operation.refresh')}
      </button>
      {historyError ? <p role="alert">{t('operation.historyError')}</p> : null}
      {history?.data.length === 0 ? <p>{t('operation.empty')}</p> : null}
      {history !== null && history.data.length > 0 ? (
        <div className="resource-operations__table">
          <table>
            <caption className="sr-only">{t('operation.history')}</caption>
            <thead>
              <tr>
                <th scope="col">{t('operation.receipt')}</th>
                <th scope="col">{t('operation.resource')}</th>
                <th scope="col">{t('operation.action')}</th>
                <th scope="col">{t('operation.updated')}</th>
              </tr>
            </thead>
            <tbody>
              {history.data.map((row) => (
                <tr key={row.id}>
                  <td>
                    <TechnicalText>{row.id}</TechnicalText>
                    <p>{t(`operation.${row.state}`)}</p>
                  </td>
                  <td>
                    <TechnicalText>{row.resource_id}</TechnicalText>
                  </td>
                  <td>
                    {t(
                      row.action === 'kubernetes.workload_scale'
                        ? 'operation.scale'
                        : 'operation.restart',
                    )}
                  </td>
                  <td>
                    {formatDate(Date.parse(row.updated_at), {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
      {history?.capped ? (
        <p>{t('operation.cap')}</p>
      ) : history?.page.has_more ? (
        <button
          type="button"
          disabled={historyBusy}
          onClick={() => {
            void loadHistory(true);
          }}
        >
          {t('operation.more')}
        </button>
      ) : null}
    </section>
  );
}
