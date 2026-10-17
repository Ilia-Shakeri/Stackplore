import type {
  ApiClient,
  DockerResourceMetadata,
  KubernetesResourceMetadata,
  ProductResource,
} from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';
import { isResourceMetadata } from './product-api-metadata';
import { createProductApi } from './product-api';

export const dockerFacts = (): DockerResourceMetadata => ({
  state: 'running',
  health: 'healthy',
  restart_count: 2,
  engine_version: '28.0.0',
  image: 'shop/api:1.0',
  started_at: '2026-08-16T08:00:00.123456Z',
  exit_code: 0,
  cpu_limit_cores: 0.5,
  memory_limit_bytes: 1048576,
  ports: [{ container_port: 8080, host_port: 8081, protocol: 'tcp', published: true }],
  mounts: [{ type: 'bind', target: '/data', read_only: true }],
  labels: [{ key: 'com.docker.compose.project', value: 'shop' }],
});
export const kubernetesFacts = (): KubernetesResourceMetadata => ({
  containers: [
    {
      name: 'checkout',
      state: 'waiting',
      ready: false,
      restart_count: 3,
      reason_code: 'kubernetes.container.crash_loop',
      cpu_request_cores: 0,
      cpu_limit_cores: 1,
      memory_request_bytes: 0,
      memory_limit_bytes: 1048576,
    },
  ],
  conditions: [{ type: 'ready', status: 'false', reason_code: 'kubernetes.condition.false' }],
});
const resource = (provider: 'docker' | 'kubernetes'): ProductResource => ({
  id: `r1_${'A'.repeat(43)}`,
  provider,
  source_id: 'source_main',
  environment_id: 'env_shop',
  kind: `${provider}.${provider === 'docker' ? 'container' : 'pod'}`,
  scope_kind: 'project',
  name: 'Checkout',
  observed_at: '2026-08-16T08:00:00Z',
  synthetic: false,
});

describe('reviewed resource metadata boundary', () => {
  it('accepts bounded provider facts, explicit zero quantities and unpublished ports', () => {
    expect(isResourceMetadata({ docker: dockerFacts() }, 'docker')).toBe(true);
    expect(isResourceMetadata({ kubernetes: kubernetesFacts() }, 'kubernetes')).toBe(true);
    expect(
      isResourceMetadata(
        {
          docker: {
            ...dockerFacts(),
            ports: [{ container_port: 80, protocol: 'udp', published: false }],
          },
        },
        'docker',
      ),
    ).toBe(true);
    expect(
      isResourceMetadata({ kubernetes: { containers: [], conditions: [] } }, 'kubernetes'),
    ).toBe(true);
  });
  it.each([
    { ...dockerFacts(), command: [] },
    { ...dockerFacts(), environment: [] },
    { ...dockerFacts(), raw_payload: {} },
    { ...dockerFacts(), ports: null },
    {
      ...dockerFacts(),
      labels: Array(17).fill({ key: 'com.docker.compose.project', value: 'shop' }),
    },
    { ...dockerFacts(), labels: [{ key: 'unreviewed', value: 'public' }] },
    { ...dockerFacts(), labels: [{ key: 'com.docker.compose.project', value: 'shop,other' }] },
    { ...dockerFacts(), labels: [{ key: 'stackplore.io/depends-on', value: 'api,,redis' }] },
    { ...dockerFacts(), labels: [dockerFacts().labels[0], dockerFacts().labels[0]] },
    {
      ...dockerFacts(),
      mounts: [{ type: 'bind', target: '/data', read_only: true, source: '/host' }],
    },
    { ...dockerFacts(), mounts: [{ type: 'bind', target: '/bad\npath', read_only: true }] },
    { ...dockerFacts(), image: 'https://user@registry/image' },
    { ...dockerFacts(), image: 'repo@sha256:invalid' },
    { ...dockerFacts(), engine_version: 'v\u202E28' },
    { ...dockerFacts(), state: 'new-state' },
    { ...dockerFacts(), restart_count: Number.MAX_SAFE_INTEGER + 1 },
    { ...dockerFacts(), exit_code: 2147483648 },
    { ...dockerFacts(), started_at: '2026-08-16T08:00:00.1234567Z' },
    { ...dockerFacts(), started_at: '2026-02-30T08:00:00Z' },
    { ...dockerFacts(), cpu_limit_cores: 0 },
    { ...dockerFacts(), cpu_limit_cores: Number.POSITIVE_INFINITY },
    { ...dockerFacts(), memory_limit_bytes: -1 },
    { ...dockerFacts(), memory_limit_bytes: 1.5 },
    { ...dockerFacts(), ports: [{ container_port: 80, protocol: 'tcp', published: true }] },
    {
      ...dockerFacts(),
      ports: [{ container_port: 80, protocol: 'tcp', published: false, host_port: 8080 }],
    },
    { ...dockerFacts(), ports: [dockerFacts().ports[0], { ...dockerFacts().ports[0] }] },
  ])('rejects unreviewed or invalid Docker facts %#', (facts) => {
    expect(isResourceMetadata({ docker: facts }, 'docker')).toBe(false);
  });
  it.each([
    { ...kubernetesFacts(), conditions: null },
    { ...kubernetesFacts(), containers: Array(65).fill(kubernetesFacts().containers[0]) },
    {
      ...kubernetesFacts(),
      containers: [kubernetesFacts().containers[0], { ...kubernetesFacts().containers[0] }],
    },
    {
      ...kubernetesFacts(),
      conditions: [kubernetesFacts().conditions[0], { ...kubernetesFacts().conditions[0] }],
    },
    { ...kubernetesFacts(), containers: [{ ...kubernetesFacts().containers[0], name: 'UPPER' }] },
    {
      ...kubernetesFacts(),
      containers: [
        {
          ...kubernetesFacts().containers[0],
          reason_code: 'kubernetes.container.provider_message',
        },
      ],
    },
    {
      ...kubernetesFacts(),
      containers: [{ ...kubernetesFacts().containers[0], cpu_limit_cores: 1e9 + 1 }],
    },
    {
      ...kubernetesFacts(),
      containers: [{ ...kubernetesFacts().containers[0], memory_request_bytes: -1 }],
    },
    { ...kubernetesFacts(), conditions: [{ type: 'ready', status: false }] },
  ])('rejects invalid Kubernetes facts %#', (facts) => {
    expect(isResourceMetadata({ kubernetes: facts }, 'kubernetes')).toBe(false);
  });
  it('rejects mixed, missing and mismatched provider envelopes', () => {
    expect(
      isResourceMetadata({ docker: dockerFacts(), kubernetes: kubernetesFacts() }, 'docker'),
    ).toBe(false);
    expect(isResourceMetadata({}, 'docker')).toBe(false);
    expect(isResourceMetadata({ docker: dockerFacts() }, 'kubernetes')).toBe(false);
    expect(isResourceMetadata({ docker: dockerFacts() }, 'demo')).toBe(false);
  });
  it('checks detail and inventory responses before exposing metadata to the view', async () => {
    const valid = { ...resource('docker'), metadata: { docker: dockerFacts() } };
    const invalid = { ...resource('kubernetes'), metadata: { docker: dockerFacts() } };
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce({ data: valid, status: 200, requestId: 'req_metadata_01' })
      .mockResolvedValueOnce({ data: invalid, status: 200, requestId: 'req_metadata_02' })
      .mockResolvedValueOnce({
        data: {
          availability: 'available',
          completeness: 'complete',
          observed_at: valid.observed_at,
          items: [invalid],
          page: { limit: 50, has_more: false },
        },
        status: 200,
        requestId: 'req_metadata_03',
      });
    const api = createProductApi({ request: request as ApiClient['request'] });
    await expect(api.getResource(valid.id)).resolves.toEqual(valid);
    await expect(api.getResource(valid.id)).rejects.toThrow('Resource response is invalid.');
    await expect(api.listResources()).rejects.toThrow('Resource response is invalid.');
  });
});
