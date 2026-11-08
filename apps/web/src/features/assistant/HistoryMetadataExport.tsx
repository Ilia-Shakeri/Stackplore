import { Button } from '@stackplore/ui';
import type { AssistantHistoryEntry, AssistantHistoryInput } from '@stackplore/api-client';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { AssistantApi, AssistantOwner } from './assistant-api';
import { freshHistoryExport } from './history-export';
import { useLocale } from '../../i18n';

interface Selection {
  readonly selected: readonly string[];
  readonly busy: boolean;
  readonly select: (id: string, checked: boolean) => void;
}

export function HistoryMetadataExport({
  api,
  owner,
  input,
  entries,
  children,
}: {
  readonly api: AssistantApi;
  readonly owner: AssistantOwner;
  readonly input: AssistantHistoryInput;
  readonly entries: readonly AssistantHistoryEntry[];
  readonly children: (selection: Selection) => ReactNode;
}) {
  const { t, formatNumber } = useLocale();
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<'error' | 'started' | null>(null);
  const work = useRef<AbortController | null>(null);
  const releaseURL = useRef<(() => void) | null>(null);

  useEffect(
    () => () => {
      work.current?.abort();
      releaseURL.current?.();
    },
    [api, owner],
  );

  const select = (id: string, checked: boolean) => {
    if (work.current !== null || !entries.some((entry) => entry.id === id)) return;
    setStatus(null);
    setSelected((current) =>
      checked
        ? [...new Set([...current, id])].slice(0, 10)
        : current.filter((value) => value !== id),
    );
  };
  const download = async () => {
    if (work.current !== null || selected.length === 0 || api.getOwner() !== owner) return;
    const controller = new AbortController();
    work.current = controller;
    setBusy(true);
    setStatus(null);
    const current = () =>
      work.current === controller && !controller.signal.aborted && api.getOwner() === owner;
    try {
      const content = await freshHistoryExport(api, owner, input, selected, controller.signal);
      if (!current()) return;
      releaseURL.current?.();
      const url = URL.createObjectURL(new Blob([content], { type: 'application/json' }));
      let timer: ReturnType<typeof setTimeout> | null = null;
      const release = () => {
        if (timer !== null) clearTimeout(timer);
        URL.revokeObjectURL(url);
        if (releaseURL.current === release) releaseURL.current = null;
      };
      releaseURL.current = release;
      const link = document.createElement('a');
      link.href = url;
      link.download = 'stackplore-run-history.json';
      document.body.append(link);
      try {
        link.click();
      } finally {
        link.remove();
      }
      timer = setTimeout(release, 1000);
      setSelected([]);
      setStatus('started');
    } catch {
      if (current()) {
        releaseURL.current?.();
        setStatus('error');
      }
    } finally {
      if (current()) {
        work.current = null;
        setBusy(false);
      }
    }
  };
  return (
    <section aria-label={t('assistant.history.export.title')} aria-busy={busy}>
      {children({ selected, busy, select })}
      <div className="assistant__actions">
        <span>
          {t('assistant.history.export.count', { number: formatNumber(selected.length) })}
        </span>
        <Button
          variant="outline"
          disabled={busy || selected.length === 0}
          onClick={() => {
            void download();
          }}
        >
          {t(busy ? 'assistant.history.export.loading' : 'assistant.history.export.action')}
        </Button>
      </div>
      <p>{t('assistant.history.export.notice')}</p>
      {status !== null ? (
        <p role={status === 'error' ? 'alert' : 'status'}>
          {t(`assistant.history.export.${status}`)}
        </p>
      ) : null}
    </section>
  );
}
