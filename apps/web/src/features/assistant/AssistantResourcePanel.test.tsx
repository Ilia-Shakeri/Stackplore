import type { AssistantResourceCitation, ProductResource } from '@stackplore/api-client';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { StrictMode } from 'react';

import { LocaleProvider, useLocale } from '../../i18n';
import { AuthProvider } from '../auth/auth-context';
import { createAuthenticatedAuthStore, createResolvedAuthStore } from '../../test/auth';
import { AssistantProvider } from './assistant-context';
import { AssistantResourcePanel } from './AssistantResourcePanel';
import { controlledConversationApi, readyResponse, scope } from './conversation.test-fixtures';

const resource: ProductResource = {
  id: scope.resourceId,
  source_id: scope.sourceId,
  provider: 'docker',
  kind: 'docker.container',
  environment_id: 'env_demo_01',
  scope_kind: 'project',
  name: 'Selected service',
  observed_at: '2026-10-28T10:15:00Z',
  synthetic: true,
};
const language = {
  en: {
    open: 'Open assistant',
    question: 'Question about this resource',
    send: 'Send question',
    facts: 'Observed facts',
    close: 'Close and clear view',
  },
  fa: {
    open: 'بازکردن دستیار',
    question: 'پرسش دربارهٔ این منبع',
    send: 'ارسال پرسش',
    facts: 'واقعیت‌های مشاهده‌شده',
    close: 'بستن و پاک‌کردن نما',
  },
} as const;
function LocaleSwitch() {
  const { locale, setLocale } = useLocale();
  return (
    <button
      onClick={() => {
        setLocale(locale === 'en' ? 'fa' : 'en');
      }}
    >
      Change test locale
    </button>
  );
}
function mount(locale: 'en' | 'fa' = 'en', guest = false) {
  const controlled = controlledConversationApi();
  const auth = guest
    ? createResolvedAuthStore({
        status: 'guest_demo',
        fixtureVersion: 'demo-v2',
        scenarioStep: 0,
        observedAt: resource.observed_at,
      })
    : createAuthenticatedAuthStore();
  const result = render(
    <StrictMode>
      <LocaleProvider defaultLocale={locale}>
        <AuthProvider store={auth}>
          <AssistantProvider api={controlled.api}>
            <h2 id="resource-detail-name" tabIndex={-1}>
              Selected service
            </h2>
            <LocaleSwitch />
            <AssistantResourcePanel resource={resource} />
          </AssistantProvider>
        </AuthProvider>
      </LocaleProvider>
    </StrictMode>,
  );
  return { ...controlled, ...result };
}
afterEach(() => {
  vi.useRealTimers();
});

describe('owning resource view', () => {
  it('keeps the guest view sealed even if a synthetic port supplies an owner', () => {
    const { api } = mount('en', true);
    expect(screen.queryByRole('button', { name: language.en.open })).toBeNull();
    expect(api.admit).not.toHaveBeenCalled();
    expect(api.get).not.toHaveBeenCalled();
  });

  it.each(['en', 'fa'] as const)(
    'opens without dispatch and completes the same explicit journey in %s',
    async (locale) => {
      const { api } = mount(locale);
      const ready = readyResponse();
      api.get.mockResolvedValue(ready);
      const words = language[locale];
      fireEvent.click(screen.getByRole('button', { name: words.open }));
      expect(api.admit).not.toHaveBeenCalled();
      expect(api.status).not.toHaveBeenCalled();
      expect(screen.getByText(scope.sourceId).getAttribute('dir')).toBe('ltr');
      expect(document.documentElement.dir).toBe(locale === 'fa' ? 'rtl' : 'ltr');
      const question =
        locale === 'fa' ? 'وضعیت این منبع را توضیح بده.' : 'Explain selected resource.';
      fireEvent.change(screen.getByRole('textbox', { name: words.question }), {
        target: { value: question },
      });
      expect(api.admit).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole('button', { name: words.send }));
      expect(await screen.findByRole('heading', { name: words.facts })).not.toBeNull();
      expect(api.admit).toHaveBeenCalledOnce();
      expect(api.admit.mock.calls[0]?.[1].locale).toBe(locale);
      expect(api.admit.mock.calls[0]?.[1].question).toBe(question);
      expect(globalThis.location.href).not.toContain(question);
      const stored = [localStorage, sessionStorage].flatMap((storage) =>
        Array.from({ length: storage.length }, (_value, index) =>
          storage.getItem(storage.key(index) ?? ''),
        ),
      );
      expect(JSON.stringify(stored)).not.toContain(question);
      fireEvent.click(screen.getByRole('button', { name: words.close }));
      expect(screen.queryByText(question)).toBeNull();
      fireEvent.click(screen.getByRole('button', { name: words.open }));
      expect(screen.getByRole('textbox', { name: words.question })).toHaveProperty('value', '');
      expect(api.admit).toHaveBeenCalledOnce();
    },
  );

  it('escapes returned text even when a controlled port bypasses reply validation', async () => {
    const { api, container } = mount();
    const ready = readyResponse();
    if (ready.content_state !== 'ready') throw new Error('Missing synthetic answer');
    const unsafe = '<img src=x onerror="window.syntheticFailure=true">';
    api.get.mockResolvedValue({
      ...ready,
      answer: { sections: [{ ...ready.answer.sections[0], text: unsafe }] },
    });
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'Explain selected resource.' },
    });
    fireEvent.click(screen.getByRole('button', { name: language.en.send }));
    expect(await screen.findByText(unsafe)).not.toBeNull();
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('script')).toBeNull();
  });

  it('erases turns on window, locale and same-account owner changes', async () => {
    const { api, setOwner } = mount();
    api.get.mockResolvedValue(readyResponse());
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    const enter = async () => {
      fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Private view fixture.' } });
      fireEvent.click(screen.getByRole('button', { name: language.en.send }));
      await screen.findByRole('heading', { name: language.en.facts });
    };
    await enter();
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '60' } });
    expect(screen.queryByText('Private view fixture.')).toBeNull();
    expect(screen.getByRole('textbox')).toHaveProperty('value', '');
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '15' } });
    await enter();
    act(() => {
      setOwner(Symbol());
    });
    expect(screen.queryByText('Private view fixture.')).toBeNull();
    await enter();
    fireEvent.click(screen.getByRole('button', { name: 'Change test locale' }));
    expect(screen.queryByText('Private view fixture.')).toBeNull();
    expect(screen.getByRole('textbox', { name: language.fa.question })).toHaveProperty('value', '');
    expect(api.admit).toHaveBeenCalledTimes(3);
  });

  it('clears the rendered draft synchronously on page exit and expiry', async () => {
    vi.useFakeTimers();
    mount();
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Private draft fixture.' } });
    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });
    expect(screen.getByRole('textbox')).toHaveProperty('value', '');
    expect(screen.queryByDisplayValue('Private draft fixture.')).toBeNull();
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Second private draft.' } });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30 * 60_000);
    });
    expect(screen.getByRole('textbox')).toHaveProperty('value', '');
  });

  it('checks citation authority afresh before focusing the selected evidence', async () => {
    const { api } = mount();
    const ready = readyResponse();
    if (ready.content_state !== 'ready') throw new Error('Missing synthetic citation');
    api.get.mockResolvedValue(ready);
    api.citation.mockResolvedValue(ready.citations[0]);
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'Explain selected resource.' },
    });
    fireEvent.click(screen.getByRole('button', { name: language.en.send }));
    const button = await screen.findByRole('button', { name: 'Check resource evidence' });
    expect(api.citation).not.toHaveBeenCalled();
    await act(async () => {
      fireEvent.click(button);
      await Promise.resolve();
    });
    expect(api.citation).toHaveBeenCalledOnce();
    expect(document.activeElement?.id).toBe('resource-detail-name');
  });

  it('disposes subscriptions and prevents late results after unmount', async () => {
    const { api, listeners, unmount } = mount();
    let finish: (value: ReturnType<typeof readyResponse>) => void = () => {
      throw new Error('Read did not start');
    };
    api.get.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Private view fixture.' } });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: language.en.send }));
      await Promise.resolve();
    });
    unmount();
    expect(listeners.size).toBe(0);
    expect(api.get.mock.calls[0]?.[2]?.aborted).toBe(true);
    await act(async () => {
      finish(readyResponse());
      await Promise.resolve();
    });
    expect(screen.queryByText('Private view fixture.')).toBeNull();
  });

  it('drops a late citation result after the evidence window changes', async () => {
    const { api } = mount();
    const ready = readyResponse();
    if (ready.content_state !== 'ready') throw new Error('Missing synthetic citation');
    let finish: (value: AssistantResourceCitation) => void = () => {
      throw new Error('Citation did not start');
    };
    api.get.mockResolvedValue(ready);
    api.citation.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'Explain selected resource.' },
    });
    fireEvent.click(screen.getByRole('button', { name: language.en.send }));
    fireEvent.click(await screen.findByRole('button', { name: 'Check resource evidence' }));
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '60' } });
    const reference = ready.citations[0];
    await act(async () => {
      finish(reference);
      await Promise.resolve();
    });
    expect(screen.queryByRole('alert')).toBeNull();
    expect(document.activeElement?.id).not.toBe('resource-detail-name');
  });

  it('shows a current citation failure and drops that warning on a locale change', async () => {
    const { api } = mount();
    api.get.mockResolvedValue(readyResponse());
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'Explain selected resource.' },
    });
    fireEvent.click(screen.getByRole('button', { name: language.en.send }));
    fireEvent.click(await screen.findByRole('button', { name: 'Check resource evidence' }));
    expect(await screen.findByRole('alert')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Change test locale' }));
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('ignores a superseded citation failure while the latest check is pending', async () => {
    const { api } = mount();
    const ready = readyResponse();
    if (ready.content_state !== 'ready') throw new Error('Missing synthetic citation');
    const reference = ready.citations[0];
    const pending: ((value: typeof reference) => void)[] = [];
    api.get.mockResolvedValue(ready);
    api.citation.mockImplementation(
      () =>
        new Promise((resolve) => {
          pending.push(resolve);
        }),
    );
    fireEvent.click(screen.getByRole('button', { name: language.en.open }));
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'Explain selected resource.' },
    });
    fireEvent.click(screen.getByRole('button', { name: language.en.send }));
    const button = await screen.findByRole('button', { name: 'Check resource evidence' });
    fireEvent.click(button);
    fireEvent.click(button);
    const first = pending[0],
      second = pending[1];
    if (first === undefined || second === undefined) throw new Error('Missing pending checks');
    await act(async () => {
      first(reference);
      await Promise.resolve();
    });
    expect(screen.queryByRole('alert')).toBeNull();
    expect(document.activeElement?.id).not.toBe('resource-detail-name');
    await act(async () => {
      second(reference);
      await Promise.resolve();
    });
    expect(screen.queryByRole('alert')).toBeNull();
    expect(document.activeElement?.id).toBe('resource-detail-name');
  });
});
