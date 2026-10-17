/// <reference types="node" />

import { createHash } from 'node:crypto';
import { runInNewContext } from 'node:vm';

import { describe, expect, it, vi } from 'vitest';

import workerSource from '../../public/sw.js?raw';
import offlineSource from '../../public/offline.html?raw';
import offlineScript from '../../public/offline.js?raw';
import en from './messages.en.json';
import fa from './messages.fa.json';

interface WorkerEvent {
  request?: { method: string; url: string; mode: string };
  data?: { type: string };
  respondWith: (response: Promise<Response>) => void;
  waitUntil: (work: Promise<unknown>) => void;
}

function createWorker() {
  const handlers = new Map<string, (event: WorkerEvent) => void>();
  const put = vi
    .fn<(_request: Request, _response: Response) => Promise<void>>()
    .mockResolvedValue();
  const match = vi.fn<(_path: string) => Promise<Response | undefined>>();
  const fetch = vi.fn<(_request: Request) => Promise<Response>>();
  const deleteCache = vi.fn<(_name: string) => Promise<boolean>>().mockResolvedValue(true);
  const skipWaiting = vi.fn<() => Promise<void>>().mockResolvedValue();
  const claim = vi.fn<() => Promise<void>>().mockResolvedValue();
  class OriginRequest extends Request {
    constructor(input: string, options: RequestInit) {
      super(new URL(input, 'https://stackplore.test'), options);
    }
  }
  runInNewContext(workerSource, {
    globalThis: {
      addEventListener: (name: string, handler: (event: WorkerEvent) => void) =>
        handlers.set(name, handler),
      caches: {
        open: vi.fn().mockResolvedValue({ put, match }),
        keys: vi.fn().mockResolvedValue(['stackplore-public-shell-0', 'unrelated-site-cache']),
        delete: deleteCache,
      },
      clients: { claim },
      location: { origin: 'https://stackplore.test' },
      fetch,
      Request: OriginRequest,
      Response,
      URL,
      skipWaiting,
    },
  });
  function dispatch(name: string, request?: WorkerEvent['request']) {
    const respondWith = vi.fn<WorkerEvent['respondWith']>();
    const waitUntil = vi.fn<WorkerEvent['waitUntil']>();
    const event = { respondWith, waitUntil, ...(request ? { request } : {}) };
    handlers.get(name)?.(event);
    return { respondWith, waitUntil };
  }
  return { handlers, put, match, fetch, deleteCache, claim, skipWaiting, dispatch };
}

describe('public offline shell privacy', () => {
  it.each([
    ['GET', '/api', 'navigate'],
    ['GET', '/api/v1/auth/session', 'navigate'],
    ['GET', '/api/v1/resources/123/logs', 'cors'],
    ['GET', '/api/v1/stream', 'cors'],
    ['GET', '/metrics', 'navigate'],
    ['GET', '/healthz', 'navigate'],
    ['GET', '/readyz', 'navigate'],
    ['POST', '/offline.html', 'cors'],
    ['GET', '/assets/resource-data.json', 'cors'],
    ['GET', '/offline.html?private=true', 'cors'],
    ['GET', 'https://foreign.test/offline.html', 'navigate'],
  ])('leaves %s %s network-only', (method, path, mode) => {
    const worker = createWorker();
    const event = worker.dispatch('fetch', {
      method,
      url: new URL(path, 'https://stackplore.test').href,
      mode,
    });
    expect(event.respondWith).not.toHaveBeenCalled();
    expect(worker.put).not.toHaveBeenCalled();
  });

  it('never stores successful private page navigation', async () => {
    const worker = createWorker();
    const response = new Response('live page', { status: 200 });
    worker.fetch.mockResolvedValue(response);
    const event = worker.dispatch('fetch', {
      method: 'GET',
      url: 'https://stackplore.test/settings',
      mode: 'navigate',
    });
    await expect(event.respondWith.mock.calls[0]?.[0]).resolves.toBe(response);
    expect(worker.put).not.toHaveBeenCalled();
  });

  it('uses only the public offline document after a failed navigation', async () => {
    const worker = createWorker();
    const offline = new Response('public offline shell');
    worker.fetch.mockRejectedValue(new TypeError('Connection lost'));
    worker.match.mockResolvedValue(offline);
    const event = worker.dispatch('fetch', {
      method: 'GET',
      url: 'https://stackplore.test/resources',
      mode: 'navigate',
    });
    await expect(event.respondWith.mock.calls[0]?.[0]).resolves.toBe(offline);
    expect(worker.match).toHaveBeenCalledWith('/offline.html');
    expect(worker.put).not.toHaveBeenCalled();
  });

  it('does not hide a server denial behind an offline page', async () => {
    const worker = createWorker();
    const denied = new Response('denied', { status: 403 });
    worker.fetch.mockResolvedValue(denied);
    const event = worker.dispatch('fetch', {
      method: 'GET',
      url: 'https://stackplore.test/settings',
      mode: 'navigate',
    });
    await expect(event.respondWith.mock.calls[0]?.[0]).resolves.toBe(denied);
    expect(worker.match).not.toHaveBeenCalled();
  });

  it.each(['private', 'no-store'])('refuses %s responses during installation', async (policy) => {
    const worker = createWorker();
    worker.fetch.mockResolvedValue(
      new Response('page', { headers: { 'Content-Type': 'text/html', 'Cache-Control': policy } }),
    );
    const event = worker.dispatch('install');
    await expect(event.waitUntil.mock.calls[0]?.[0]).rejects.toThrow(
      'Public offline shell is unavailable.',
    );
    expect(worker.put).not.toHaveBeenCalled();
  });

  it('precaches only named public files and sends no credentials', async () => {
    const worker = createWorker();
    const types = ['text/html', 'text/css', 'application/javascript', 'image/svg+xml'];
    worker.fetch.mockImplementation(() =>
      Promise.resolve(
        new Response('public shell', { headers: { 'Content-Type': types.shift() ?? '' } }),
      ),
    );
    const event = worker.dispatch('install');
    await event.waitUntil.mock.calls[0]?.[0];
    expect(worker.fetch.mock.calls.map(([request]) => new URL(request.url).pathname)).toEqual([
      '/offline.html',
      '/offline.css',
      '/offline.js',
      '/brand/stackplore-favicon.svg',
    ]);
    expect(worker.fetch.mock.calls.every(([request]) => request.credentials === 'omit')).toBe(true);
    expect(worker.put).toHaveBeenCalledTimes(4);
    expect(worker.skipWaiting).not.toHaveBeenCalled();
  });

  it('cleans only its own old public shell caches', async () => {
    const worker = createWorker();
    const event = worker.dispatch('activate');
    await event.waitUntil.mock.calls[0]?.[0];
    expect(worker.deleteCache.mock.calls).toEqual([['stackplore-public-shell-0']]);
    expect(worker.claim).toHaveBeenCalledOnce();
  });

  it('ships bilingual offline text and a matching hash-based script policy', () => {
    expect(offlineSource).toContain('lang="fa" dir="rtl"');
    expect(offlineSource).toContain('Infrastructure data is never saved');
    const hash = createHash('sha256').update(offlineScript).digest('base64');
    expect(offlineSource).toContain(`'sha256-${hash}'`);
    expect(offlineSource).toContain(`integrity="sha256-${hash}"`);
    expect(Object.keys(fa).sort()).toEqual(Object.keys(en).sort());
  });
});
