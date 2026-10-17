import { afterEach, describe, expect, it, vi } from 'vitest';

import { registerPublicShell } from './register';

function workerRegistration(waiting = true) {
  const registration = new EventTarget();
  const container = new EventTarget();
  const postMessage = vi.fn();
  const installed = new EventTarget();
  const register = vi.fn().mockResolvedValue(registration);
  Object.assign(registration, { waiting: waiting ? { postMessage } : null, installing: installed });
  Object.assign(installed, { state: 'installing', postMessage });
  Object.assign(container, { register, controller: {} });
  return {
    registration,
    container: container as ServiceWorkerContainer,
    postMessage,
    register,
    installed,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('public shell update registration', () => {
  it('does not register outside a secure context or unsupported browser', async () => {
    vi.stubGlobal('isSecureContext', false);
    const worker = workerRegistration();
    await expect(registerPublicShell(vi.fn(), worker.container)).resolves.toBeUndefined();
    expect(worker.register).not.toHaveBeenCalled();
  });

  it('announces updates but reloads only after explicit acceptance', async () => {
    vi.stubGlobal('isSecureContext', true);
    const worker = workerRegistration();
    const onUpdate = vi.fn();
    const reload = vi.fn();
    const result = await registerPublicShell(onUpdate, worker.container, reload);
    expect(worker.register).toHaveBeenCalledWith('/sw.js', { scope: '/', updateViaCache: 'none' });
    expect(onUpdate).toHaveBeenCalledOnce();
    worker.container.dispatchEvent(new Event('controllerchange'));
    expect(reload).not.toHaveBeenCalled();
    expect(result?.activateUpdate()).toBe(true);
    expect(worker.postMessage).toHaveBeenCalledWith({ type: 'ACTIVATE_PUBLIC_SHELL' });
    worker.container.dispatchEvent(new Event('controllerchange'));
    worker.container.dispatchEvent(new Event('controllerchange'));
    expect(reload).toHaveBeenCalledOnce();
  });

  it('releases listeners and cannot activate or reload after disposal', async () => {
    vi.stubGlobal('isSecureContext', true);
    const worker = workerRegistration();
    const reload = vi.fn();
    const onUpdate = vi.fn();
    const result = await registerPublicShell(onUpdate, worker.container, reload);
    result?.activateUpdate();
    result?.dispose();
    worker.container.dispatchEvent(new Event('controllerchange'));
    Object.assign(worker.installed, { state: 'installed' });
    worker.installed.dispatchEvent(new Event('statechange'));
    expect(result?.activateUpdate()).toBe(false);
    expect(reload).not.toHaveBeenCalled();
    expect(onUpdate).toHaveBeenCalledOnce();
  });

  it('accepts the installed worker before the waiting property becomes visible', async () => {
    vi.stubGlobal('isSecureContext', true);
    const worker = workerRegistration(false);
    const onUpdate = vi.fn();
    const reload = vi.fn();
    const result = await registerPublicShell(onUpdate, worker.container, reload);
    expect(onUpdate).not.toHaveBeenCalled();
    expect(result?.activateUpdate()).toBe(false);
    Object.assign(worker.installed, { state: 'installed' });
    worker.installed.dispatchEvent(new Event('statechange'));
    expect(onUpdate).toHaveBeenCalledOnce();
    expect(reload).not.toHaveBeenCalled();
    expect(result?.activateUpdate()).toBe(true);
    expect(worker.postMessage).toHaveBeenCalledWith({ type: 'ACTIVATE_PUBLIC_SHELL' });
    worker.container.dispatchEvent(new Event('controllerchange'));
    expect(reload).toHaveBeenCalledOnce();
    result?.dispose();
  });
});
