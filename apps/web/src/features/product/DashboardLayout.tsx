import { Children, useState, type ReactNode } from 'react';
import { useLocale } from '../../i18n';
import { useOptionalAuthSnapshot } from '../auth/auth-context';

const titles = [
  'product.trend.cpu',
  'product.trend.memory',
  'insight.restarts',
  'insight.receive',
  'insight.send',
] as const;
type Panel = { id: number; visible: boolean; wide: boolean };
const defaults = (): Panel[] => titles.map((_, id) => ({ id, visible: true, wide: false }));

const load = (storageKey: string | null): Panel[] => {
  try {
    if (storageKey === null) return defaults();
    const raw = localStorage.getItem(storageKey);
    if (raw === null || raw.length > 2048) return defaults();
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value) || value.length !== titles.length) return defaults();
    const seen = new Set<number>();
    for (const panel of value as unknown[]) {
      if (typeof panel !== 'object' || panel === null) return defaults();
      const item = panel as Record<string, unknown>;
      if (
        Object.keys(item).length !== 3 ||
        typeof item.id !== 'number' ||
        !Number.isInteger(item.id) ||
        item.id < 0 ||
        item.id >= titles.length ||
        seen.has(item.id) ||
        typeof item.visible !== 'boolean' ||
        typeof item.wide !== 'boolean'
      )
        return defaults();
      seen.add(item.id);
    }
    return value as Panel[];
  } catch {
    return defaults();
  }
};

export function DashboardLayout({ children }: { readonly children: ReactNode }) {
  const snapshot = useOptionalAuthSnapshot();
  const storageKey =
    snapshot?.status === 'authenticated'
      ? `stackplore.dashboard.layout.v2.user.${encodeURIComponent(snapshot.user.id)}`
      : snapshot?.status === 'guest_demo'
        ? 'stackplore.dashboard.layout.v2.guest'
        : null;
  return (
    <BrowserDashboardLayout key={storageKey ?? 'volatile'} storageKey={storageKey}>
      {children}
    </BrowserDashboardLayout>
  );
}

export function BrowserDashboardLayout({
  children,
  storageKey,
}: {
  readonly children: ReactNode;
  readonly storageKey: string | null;
}) {
  const { t } = useLocale();
  const [layout, setLayout] = useState(() => load(storageKey));
  const [saved, setSaved] = useState(true);
  const panels = Children.toArray(children);
  const update = (next: Panel[]) => {
    setLayout(next);
    try {
      if (storageKey === null) {
        setSaved(false);
        return;
      }
      localStorage.setItem(storageKey, JSON.stringify(next));
      setSaved(true);
    } catch {
      setSaved(false);
    }
  };
  const move = (index: number, direction: number) => {
    const next = [...layout];
    const target = index + direction;
    const current = next[index],
      other = next[target];
    if (current === undefined || other === undefined) return;
    next[index] = other;
    next[target] = current;
    update(next);
  };
  return (
    <section aria-label={t('dashboard.title')}>
      <details className="dashboard-editor">
        <summary>{t('dashboard.customize')}</summary>
        <p>{t('dashboard.browserOnly')}</p>
        <ul>
          {layout.map((panel, index) => (
            <li key={panel.id}>
              <strong>{t(titles[panel.id] ?? 'product.trend.cpu')}</strong>
              <label>
                <input
                  type="checkbox"
                  checked={panel.visible}
                  onChange={() =>
                    update(
                      layout.map((item) =>
                        item.id === panel.id ? { ...item, visible: !item.visible } : item,
                      ),
                    )
                  }
                />
                {t('dashboard.visible')}
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={panel.wide}
                  onChange={() =>
                    update(
                      layout.map((item) =>
                        item.id === panel.id ? { ...item, wide: !item.wide } : item,
                      ),
                    )
                  }
                />
                {t('dashboard.wide')}
              </label>
              <button
                type="button"
                disabled={index === 0}
                onClick={() => move(index, -1)}
                aria-label={`${t('dashboard.up')}: ${t(titles[panel.id] ?? 'product.trend.cpu')}`}
              >
                {t('dashboard.up')}
              </button>
              <button
                type="button"
                disabled={index === layout.length - 1}
                onClick={() => move(index, 1)}
                aria-label={`${t('dashboard.down')}: ${t(titles[panel.id] ?? 'product.trend.cpu')}`}
              >
                {t('dashboard.down')}
              </button>
            </li>
          ))}
        </ul>
        <button type="button" onClick={() => update(defaults())}>
          {t('dashboard.reset')}
        </button>
      </details>
      {!saved ? <p role="status">{t('dashboard.notSaved')}</p> : null}
      {layout.every((panel) => !panel.visible) ? (
        <div role="status" className="trend-card">
          <p>{t('dashboard.empty')}</p>
          <button type="button" onClick={() => update(defaults())}>{t('dashboard.restore')}</button>
        </div>
      ) : null}
      <div className="trend-grid">
        {layout
          .filter((panel) => panel.visible)
          .map((panel) => (
            <div
              key={panel.id}
              className={panel.wide ? 'dashboard-panel dashboard-panel--wide' : 'dashboard-panel'}
            >
              {panels[panel.id]}
            </div>
          ))}
      </div>
    </section>
  );
}
