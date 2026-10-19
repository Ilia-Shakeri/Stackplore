/// <reference types="node" />

import { runInNewContext } from 'node:vm';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import initializerSource from '../../public/theme-init.js?raw';

const themeSource = readFileSync(
  resolve(process.cwd(), '../../packages/ui/src/styles/themes.css'),
  'utf8',
);

function themeValues(pattern: RegExp): Record<string, string> {
  const block = pattern.exec(themeSource)?.[1];
  expect(block).toBeDefined();
  return Object.fromEntries(
    [...(block ?? '').matchAll(/(--sp-[\w-]+):\s*([^;]+);/gu)].map((match) => [
      match[1] ?? '',
      match[2]?.trim() ?? '',
    ]),
  );
}

function luminance(hex: string): number {
  if (!/^#[0-9a-f]{6}$/iu.test(hex)) throw new Error('Expected a palette color.');
  const channel = (start: number) => {
    const value = Number.parseInt(hex.slice(start, start + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5);
}

function contrast(first: string, second: string): number {
  const values = [luminance(first), luminance(second)].sort((left, right) => left - right);
  return ((values[1] ?? 0) + 0.05) / ((values[0] ?? 0) + 0.05);
}

describe('brand palette and first paint', () => {
  const light = themeValues(/\[data-theme='light'\]\s*\{([^}]+)\}/u);
  const dark = themeValues(/\[data-theme='dark'\]\s*\{([^}]+)\}/u);
  const systemDark = themeValues(/:root:not\(\[data-theme\]\)\s*\{([^}]+)\}/u);

  it.each([
    ['light', light],
    ['dark', dark],
  ] as const)('keeps readable text, actions, status and focus in %s', (_name, palette) => {
    const color = (token: string) => palette[token] ?? '';
    for (const surface of ['--sp-bg', '--sp-surface', '--sp-surface-raised']) {
      for (const text of ['--sp-text', '--sp-text-muted', '--sp-text-subtle', '--sp-primary']) {
        expect(
          contrast(color(text), color(surface)),
          `${text} on ${surface}`,
        ).toBeGreaterThanOrEqual(4.5);
      }
      expect(contrast(color('--sp-focus'), color(surface))).toBeGreaterThanOrEqual(3);
      expect(contrast(color('--sp-border-strong'), color(surface))).toBeGreaterThanOrEqual(3);
    }
    for (const primary of ['--sp-primary', '--sp-primary-hover', '--sp-primary-active']) {
      expect(contrast(color('--sp-on-primary'), color(primary))).toBeGreaterThanOrEqual(4.5);
    }
    for (const status of ['info', 'success', 'warning', 'danger']) {
      expect(
        contrast(color(`--sp-${status}`), color(`--sp-${status}-surface`)),
        `${status} status`,
      ).toBeGreaterThanOrEqual(4.5);
    }
  });

  it('uses the same dark palette for an explicit or system preference', () => {
    expect(systemDark).toEqual(dark);
  });

  it.each([
    ['light', true, 'light', '#f4f8fd'],
    ['dark', false, 'dark', '#06111f'],
    ['system', true, 'dark', '#06111f'],
    ['system', false, 'light', '#f4f8fd'],
  ])('resolves %s with system dark=%s before paint', (stored, systemDark, resolved, color) => {
    const root = { dataset: {} as Record<string, string>, style: { colorScheme: '' } };
    let metaColor = '';
    runInNewContext(initializerSource, {
      globalThis: {
        localStorage: { getItem: () => stored },
        matchMedia: () => ({ matches: systemDark }),
        document: {
          documentElement: root,
          querySelector: () => ({
            setAttribute: (_key: string, value: string) => {
              metaColor = value;
            },
          }),
        },
      },
    });
    expect(root.dataset.theme).toBe(resolved);
    expect(root.dataset.themePreference).toBe(stored);
    expect(root.style.colorScheme).toBe(resolved);
    expect(metaColor).toBe(color);
  });
});
