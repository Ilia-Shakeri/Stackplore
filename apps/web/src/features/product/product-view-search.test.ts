import { describe, expect, it } from 'vitest';
import { productViewSearch, validViewParameter } from './product-view-search';

describe('shared exploration query policy', () => {
  it.each([
    ['event_q', 200],
    ['log_q', 200],
    ['scope_name', 256],
    ['topology_scope', 300],
  ] as const)('counts supplementary text by code points for %s', (name, maximum) => {
    expect(validViewParameter(name, '🚀'.repeat(maximum))).toBe(true);
    expect(validViewParameter(name, '🚀'.repeat(maximum + 1))).toBe(false);
    expect(productViewSearch({ [name]: '🚀'.repeat(maximum) })).toEqual({
      [name]: '🚀'.repeat(maximum),
    });
  });
  it.each(['trace', 'unknown'])('preserves the supported log severity %s', (severity) => {
    expect(validViewParameter('severity', severity)).toBe(true);
    expect(productViewSearch({ severity })).toEqual({ severity });
    expect(validViewParameter('severity', `${severity}_invalid`)).toBe(false);
  });
  it('accepts fixed safe filters and canonicalizes parsed booleans and numbers', () => {
    expect(
      productViewSearch({
        previous: true,
        wrap: false,
        minutes: 360,
        container: '1_worker.main',
        health: 'healthy',
        topology_scope: 'project: فروشگاه',
        rule_cursor: 'cursor_123',
        unknown: 'ignored',
      }),
    ).toEqual({
      previous: 'true',
      wrap: 'false',
      minutes: '360',
      container: '1_worker.main',
      health: 'healthy',
      topology_scope: 'project: فروشگاه',
      rule_cursor: 'cursor_123',
    });
  });
  it('enforces the transport boundary lengths, timestamp precision and closed names', () => {
    expect(validViewParameter('container', `1${'x'.repeat(255)}`)).toBe(true);
    expect(validViewParameter('container', `1${'x'.repeat(256)}`)).toBe(false);
    expect(validViewParameter('environment_id', `e${'x'.repeat(128)}`)).toBe(false);
    expect(validViewParameter('scope_name', 'x'.repeat(256))).toBe(true);
    expect(validViewParameter('topology_scope', `namespace: ${'x'.repeat(253)}`)).toBe(true);
    expect(
      productViewSearch({ topology_scope: `namespace: ${'x'.repeat(253)}` }).topology_scope,
    ).toBe(`namespace: ${'x'.repeat(253)}`);
    expect(validViewParameter('from', '2026-08-16T08:00:00.123456Z')).toBe(true);
    expect(validViewParameter('from', '2026-08-16T08:00:00.1234567Z')).toBe(false);
    expect(validViewParameter('kind', 'docker.container')).toBe(true);
    expect(validViewParameter('kind', 'docker')).toBe(false);
    expect(validViewParameter('endpoint', 'https://example.test')).toBe(false);
    expect(validViewParameter('log_q', 'line\nnext')).toBe(false);
  });
});
