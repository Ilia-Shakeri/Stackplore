import { useEffect, useMemo, useState } from 'react';
import { useLocale } from '../../i18n';
import { useExplorationView } from './exploration-ui';
import { useProductApi, useProductSource } from './product-context';

export function useExplorationTime(revision = 0, live = false) {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const [sourceClock, setSourceClock] = useState<number | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    setSourceClock(null);
    void api.listSources(controller.signal).then(
      (sources) => {
        if (controller.signal.aborted) return;
        const source =
          selectedSource === undefined
            ? sources[0]
            : sources.find((item) => item.id === selectedSource);
        if (source?.synthetic === true && Number.isFinite(Date.parse(source.observed_at)))
          setSourceClock(Date.parse(source.observed_at) + 1);
      },
      () => undefined,
    );
    return () => {
      controller.abort();
    };
  }, [api, selectedSource]);
  const view = useExplorationView();
  const from = view.read('from', 30);
  const to = view.read('to', 30);
  const viewMinutes = view.read('minutes');
  const [minutes, setMinutes] = useState(() => {
    const value = Number(viewMinutes);
    return value === 360 || value === 1440 ? value : 60;
  });
  useEffect(() => {
    const value = Number(viewMinutes);
    setMinutes(value === 360 || value === 1440 ? value : 60);
  }, [viewMinutes]);
  const range = useMemo(() => {
    if (
      !live &&
      /^\d{4}-\d\d-\d\dT/u.test(from) &&
      /^\d{4}-\d\d-\d\dT/u.test(to) &&
      Number.isFinite(Date.parse(from)) &&
      Number.isFinite(Date.parse(to)) &&
      Date.parse(from) < Date.parse(to) &&
      Date.parse(to) - Date.parse(from) <= 86400000
    )
      return { from, to };
    const now = sourceClock ?? Date.now();
    return { from: new Date(now - minutes * 60000).toISOString(), to: new Date(now).toISOString() };
  }, [from, to, minutes, revision, sourceClock, live]);
  const select = (value: number) => {
    const now = sourceClock ?? Date.now();
    setMinutes(value);
    view.write('minutes', String(value));
    view.write('from', live ? '' : new Date(now - value * 60000).toISOString());
    view.write('to', live ? '' : new Date(now).toISOString());
  };
  const stableRange = useMemo(() => ({ from: range.from, to: range.to }), [range.from, range.to]);
  return { minutes, range: stableRange, select };
}

export function TimeRangeControl({
  minutes,
  select,
}: {
  readonly minutes: number;
  readonly select: (value: number) => void;
}) {
  const { t } = useLocale();
  return (
    <label>
      {t('explore.range.label')}
      <select
        value={minutes}
        onChange={(event) => {
          select(Number(event.target.value));
        }}
      >
        <option value="60">{t('explore.range.hour')}</option>
        <option value="360">{t('explore.range.sixHours')}</option>
        <option value="1440">{t('explore.range.day')}</option>
      </select>
    </label>
  );
}
