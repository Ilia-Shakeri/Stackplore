import { ErrorBoundary, ThemeProvider, ToastProvider, useTheme, useToast } from '@stackplore/ui';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';

interface ToastHarnessProps {
  onAction: () => void;
}

function ToastHarness({ onAction }: ToastHarnessProps) {
  const { notify } = useToast();

  return (
    <>
      <button type="button" onClick={() => notify({ duration: 60_000, title: 'Notice one' })}>
        Add one
      </button>
      <button type="button" onClick={() => notify({ duration: 60_000, title: 'Notice two' })}>
        Add two
      </button>
      <button type="button" onClick={() => notify({ duration: 60_000, title: 'Notice three' })}>
        Add three
      </button>
      <button
        type="button"
        onClick={() =>
          notify({
            action: { label: 'Undo', onClick: onAction },
            description: 'Latest state is ready.',
            duration: 60_000,
            title: 'Notice four',
            tone: 'success',
          })
        }
      >
        Add four
      </button>
    </>
  );
}

describe('toast state', () => {
  it('keeps the newest three notices and runs an explicit action', () => {
    const handleAction = vi.fn();

    render(
      <ToastProvider closeLabel="Dismiss notice" viewportLabel="Notices">
        <ToastHarness onAction={handleAction} />
      </ToastProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Add one' }));
    fireEvent.click(screen.getByRole('button', { name: 'Add two' }));
    fireEvent.click(screen.getByRole('button', { name: 'Add three' }));
    fireEvent.click(screen.getByRole('button', { name: 'Add four' }));

    expect(screen.queryByText('Notice one')).toBeNull();
    expect(screen.getByText('Notice two')).not.toBeNull();
    expect(screen.getByText('Notice three')).not.toBeNull();
    expect(screen.getByText('Notice four').closest('[data-tone]')?.getAttribute('data-tone')).toBe(
      'success',
    );
    expect(screen.getByText('Latest state is ready.')).not.toBeNull();
    expect(screen.getAllByRole('button', { name: 'Dismiss notice' })).toHaveLength(3);

    fireEvent.click(screen.getByRole('button', { name: 'Undo' }));

    expect(handleAction).toHaveBeenCalledOnce();
    expect(screen.queryByText('Notice four')).toBeNull();
    expect(screen.getAllByRole('button', { name: 'Dismiss notice' })).toHaveLength(2);
  });
});

interface FailureProps {
  active: boolean;
}

function Failure({ active }: FailureProps) {
  if (active) {
    throw new Error('Test failure');
  }

  return <p>Recovered content</p>;
}

function ErrorBoundaryHarness() {
  const [failed, setFailed] = useState(true);

  return (
    <ErrorBoundary
      fallback={({ error, resetErrorBoundary }) => (
        <div role="alert">
          <p>{error.message}</p>
          <button type="button" onClick={resetErrorBoundary}>
            Retry
          </button>
        </div>
      )}
      onReset={() => {
        setFailed(false);
      }}
    >
      <Failure active={failed} />
    </ErrorBoundary>
  );
}

describe('error boundary state', () => {
  it('runs reset work and renders children again', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);

    render(<ErrorBoundaryHarness />);

    expect(screen.getByRole('alert').textContent).toContain('Test failure');

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByText('Recovered content')).not.toBeNull();
  });
});

function ThemeHarness() {
  const { resolvedTheme, setTheme, theme } = useTheme();

  return (
    <>
      <output data-testid="theme-value">
        {theme}:{resolvedTheme}
      </output>
      <button
        type="button"
        onClick={() => {
          setTheme('light');
        }}
      >
        Use light
      </button>
    </>
  );
}

function createMediaQuery(matches: boolean): MediaQueryList {
  return {
    matches,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addEventListener: vi.fn(),
    addListener: vi.fn(),
    dispatchEvent: vi.fn(() => false),
    removeEventListener: vi.fn(),
    removeListener: vi.fn(),
  };
}

describe('theme state', () => {
  it('reads and updates a stored preference', () => {
    globalThis.localStorage.setItem('test.theme', 'dark');

    render(
      <ThemeProvider storageKey="test.theme">
        <ThemeHarness />
      </ThemeProvider>,
    );

    expect(screen.getByTestId('theme-value').textContent).toBe('dark:dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(document.documentElement.dataset.themePreference).toBe('dark');

    fireEvent.click(screen.getByRole('button', { name: 'Use light' }));

    expect(globalThis.localStorage.getItem('test.theme')).toBe('light');
    expect(screen.getByTestId('theme-value').textContent).toBe('light:light');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('resolves the system preference', () => {
    vi.spyOn(globalThis, 'matchMedia').mockReturnValue(createMediaQuery(true));

    render(
      <ThemeProvider defaultTheme="system" storageKey="test.system-theme">
        <ThemeHarness />
      </ThemeProvider>,
    );

    expect(screen.getByTestId('theme-value').textContent).toBe('system:dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');

    act(() => {
      globalThis.dispatchEvent(
        new StorageEvent('storage', {
          key: 'test.system-theme',
          newValue: 'light',
        }),
      );
    });

    expect(screen.getByTestId('theme-value').textContent).toBe('light:light');
  });
});
