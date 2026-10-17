import type { ProductResource } from '@stackplore/api-client';
import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { ResourceMetadataPanel } from './ResourceMetadataPanel';

const resource: ProductResource = {
  id: `r1_${'A'.repeat(43)}`,
  provider: 'kubernetes',
  source_id: 'source_main',
  environment_id: 'env_shop',
  kind: 'kubernetes.pod',
  scope_kind: 'namespace',
  scope_name: 'shop',
  name: 'Checkout',
  observed_at: '2026-08-16T08:00:00Z',
  synthetic: false,
};

describe('safe resource configuration', () => {
  it.each([
    ['en', 'Observed configuration', 'Previous container logs', 'Repeated crashes', '0 cores'],
    ['fa', 'پیکربندی مشاهده‌شده', 'گزارش‌های کانتینر قبلی', 'خرابی پیاپی', '۰ هسته'],
  ] as const)(
    'renders localized container facts and exact log links in %s',
    (locale, title, previous, reason, zero) => {
      render(
        <LocaleProvider defaultLocale={locale}>
          <ResourceMetadataPanel
            resource={{
              ...resource,
              metadata: {
                kubernetes: {
                  containers: [
                    {
                      name: 'checkout',
                      state: 'waiting',
                      ready: false,
                      restart_count: 3,
                      reason_code: 'kubernetes.container.crash_loop',
                      cpu_request_cores: 0,
                    },
                  ],
                  conditions: [{ type: 'ready', status: 'false' }],
                },
              },
            }}
          />
        </LocaleProvider>,
      );
      expect(screen.getByRole('region', { name: title })).not.toBeNull();
      expect(screen.getByText('checkout').getAttribute('dir')).toBe('ltr');
      expect(screen.getByText(reason)).not.toBeNull();
      expect(screen.getByText(zero)).not.toBeNull();
      const href = screen.getByRole('link', { name: previous }).getAttribute('href');
      expect(href).toBe(
        `/logs?source=source_main&resource_id=${resource.id}&container=checkout&previous=true`,
      );
      expect(document.documentElement.dir).toBe(locale === 'fa' ? 'rtl' : 'ltr');
    },
  );
  it('renders Docker ports and mount targets with technical direction and real zero exit code', () => {
    render(
      <LocaleProvider defaultLocale="en">
        <ResourceMetadataPanel
          resource={{
            ...resource,
            provider: 'docker',
            kind: 'docker.container',
            metadata: {
              docker: {
                state: 'running',
                health: 'none',
                restart_count: 0,
                exit_code: 0,
                ports: [{ container_port: 80, host_port: 8080, protocol: 'tcp', published: true }],
                mounts: [{ type: 'volume', target: '/var/lib/app', read_only: true }],
                labels: [],
              },
            },
          }}
        />
      </LocaleProvider>,
    );
    expect(screen.getByText('80/tcp → 8080').getAttribute('dir')).toBe('ltr');
    expect(screen.getByText('/var/lib/app').getAttribute('dir')).toBe('ltr');
    expect(screen.getByText(/Read only/u)).not.toBeNull();
    const exit = screen.getByText('Exit code').parentElement;
    expect(exit).not.toBeNull();
    if (exit === null) throw new Error('Exit code fact is absent.');
    expect(within(exit).getByText('0')).not.toBeNull();
    expect(screen.queryByRole('link')).toBeNull();
  });
  it('keeps absent facts unknown and does not invent empty configuration or logs', () => {
    render(
      <LocaleProvider defaultLocale="en">
        <ResourceMetadataPanel resource={resource} />
      </LocaleProvider>,
    );
    expect(
      screen.getByText('Configuration facts were not reported for this resource.'),
    ).not.toBeNull();
    expect(screen.queryByRole('link')).toBeNull();
    expect(screen.queryByText('No items reported.')).toBeNull();
  });
  it('keeps Docker engine placeholders separate from observed container counters', () => {
    render(
      <LocaleProvider defaultLocale="en">
        <ResourceMetadataPanel
          resource={{
            ...resource,
            provider: 'docker',
            kind: 'docker.engine',
            metadata: {
              docker: {
                engine_version: '28.0.0',
                state: 'unknown',
                health: 'unknown',
                restart_count: 0,
                ports: [],
                mounts: [],
                labels: [],
              },
            },
          }}
        />
      </LocaleProvider>,
    );
    expect(screen.getByText('28.0.0').getAttribute('dir')).toBe('ltr');
    expect(screen.queryByText('Restarts')).toBeNull();
    expect(screen.queryByText('Exit code')).toBeNull();
    expect(screen.queryByText('Ports')).toBeNull();
    expect(screen.queryByText('0')).toBeNull();
  });
  it('keeps template runtime fields unknown while preserving configured quantities', () => {
    render(
      <LocaleProvider defaultLocale="en">
        <ResourceMetadataPanel
          resource={{
            ...resource,
            kind: 'kubernetes.deployment',
            metadata: {
              kubernetes: {
                containers: [
                  {
                    name: 'checkout',
                    state: 'unknown',
                    ready: false,
                    restart_count: 0,
                    cpu_request_cores: 0,
                  },
                ],
                conditions: [],
              },
            },
          }}
        />
      </LocaleProvider>,
    );
    for (const label of ['Ready', 'Restarts']) {
      const fact = screen.getByText(label).parentElement;
      if (fact === null) throw new Error('Runtime fact is absent.');
      expect(within(fact).getByText('Not reported')).not.toBeNull();
    }
    expect(screen.getByText('0 cores')).not.toBeNull();
    expect(screen.queryByRole('link')).toBeNull();
  });
  it('offers observed container-resource logs with exact identity and no previous logs before restart', () => {
    render(
      <LocaleProvider defaultLocale="en">
        <ResourceMetadataPanel
          resource={{
            ...resource,
            kind: 'kubernetes.container',
            metadata: {
              kubernetes: {
                containers: [{ name: 'checkout', state: 'running', ready: true, restart_count: 0 }],
                conditions: [],
              },
            },
          }}
        />
      </LocaleProvider>,
    );
    expect(screen.getByRole('link', { name: 'Current container logs' }).getAttribute('href')).toBe(
      `/logs?source=source_main&resource_id=${resource.id}&container=checkout`,
    );
    expect(screen.queryByRole('link', { name: 'Previous container logs' })).toBeNull();
  });
});
