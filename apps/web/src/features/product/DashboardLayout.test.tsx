import { fireEvent, render, screen, within, cleanup } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { BrowserDashboardLayout as DashboardLayout } from './DashboardLayout';

const key = 'stackplore.dashboard.layout.v1';
const show = (ownerKey: string | null = key) =>
  render(
    <LocaleProvider defaultLocale="en" storageKey="dashboard-test">
      <DashboardLayout storageKey={ownerKey}>
        {['cpu', 'memory', 'restarts', 'received', 'sent'].map((name) => (
          <p key={name}>{name}</p>
        ))}
      </DashboardLayout>
    </LocaleProvider>,
  );
afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe('dashboard layout', () => {
  it('offers recovery when all panels are hidden', () => {
    localStorage.setItem(
      key,
      JSON.stringify(Array.from({ length: 5 }, (_, id) => ({ id, visible: false, wide: false }))),
    );
    show();
    expect(screen.getByRole('status').textContent).toContain('All panels are hidden');
    fireEvent.click(screen.getByRole('button', { name: 'Restore panels' }));
    expect(screen.getByText('cpu')).not.toBeNull();
    expect(screen.queryByRole('button', { name: 'Restore panels' })).toBeNull();
  });
  it('does not reuse a different owner layout', () => {
    localStorage.setItem(
      'owner-first',
      JSON.stringify(
        Array.from({ length: 5 }, (_, id) => ({ id, visible: id !== 0, wide: false })),
      ),
    );
    const first = show('owner-first');
    expect(screen.queryByText('cpu')).toBeNull();
    first.unmount();
    const second = show('owner-second');
    expect(screen.getByText('cpu')).not.toBeNull();
    second.unmount();
    show(null);
    expect(screen.getByText('cpu')).not.toBeNull();
  });
  it('persists only bounded layout and supports reset', () => {
    const view = show();
    fireEvent.click(screen.getByText('Customize dashboard'));
    const item = screen.getAllByRole('listitem')[0];
    if (item === undefined) throw new Error('missing layout editor');
    fireEvent.click(within(item).getByLabelText('Show panel'));
    expect(screen.queryByText('cpu')).toBeNull();
    const stored: unknown = JSON.parse(localStorage.getItem(key) ?? 'null');
    expect(stored).toEqual(expect.arrayContaining([{ id: 0, visible: false, wide: false }]));
    view.unmount();
    show();
    expect(screen.queryByText('cpu')).toBeNull();
    fireEvent.click(screen.getByText('Customize dashboard'));
    fireEvent.click(screen.getByText('Reset layout'));
    expect(screen.getByText('cpu')).not.toBeNull();
  });
  it('rejects malformed saved layouts and changes order and width', () => {
    localStorage.setItem(key, JSON.stringify([{ id: 0, visible: true, wide: false }]));
    const { container } = show();
    fireEvent.click(screen.getByText('Customize dashboard'));
    fireEvent.click(screen.getByRole('button', { name: /Move later:.*CPU/ }));
    expect(container.querySelector('.trend-grid')?.textContent.startsWith('memory')).toBe(true);
    const item = screen.getAllByRole('listitem')[0];
    if (item === undefined) throw new Error('missing layout editor');
    fireEvent.click(within(item).getByLabelText('Full width'));
    expect(container.querySelector('.dashboard-panel--wide')?.textContent).toBe('memory');
  });
});
