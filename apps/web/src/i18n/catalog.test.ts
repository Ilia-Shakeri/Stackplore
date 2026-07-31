import { describe, expect, it } from 'vitest';

import { catalogs, translateText, type CatalogMessage, type TextMessageKey } from './index';

const invariantKeys = new Set([
  'route.about.eyebrow',
  'shell.locale.option.en',
  'shell.locale.option.fa',
]);

function asCatalog(value: object): Readonly<Record<string, CatalogMessage>> {
  return value as Readonly<Record<string, CatalogMessage>>;
}

function messageParts(message: CatalogMessage): readonly string[] {
  return typeof message === 'string' ? [message] : Object.values(message);
}

describe('locale catalogs', () => {
  it('keeps exact key, message-kind, plural, and placeholder parity', () => {
    const english = asCatalog(catalogs.en);
    const persian = asCatalog(catalogs.fa);
    const englishKeys = Object.keys(english).sort();
    const persianKeys = Object.keys(persian).sort();

    expect(persianKeys).toEqual(englishKeys);

    for (const key of englishKeys) {
      const englishMessage = english[key];
      const persianMessage = persian[key];

      expect(englishMessage).toBeDefined();
      expect(persianMessage).toBeDefined();
      expect(typeof persianMessage).toBe(typeof englishMessage);

      if (
        englishMessage === undefined ||
        persianMessage === undefined ||
        typeof englishMessage === 'string' ||
        typeof persianMessage === 'string'
      ) {
        continue;
      }

      expect(Object.keys(persianMessage).sort()).toEqual(Object.keys(englishMessage).sort());
      expect(englishMessage.other).not.toBe('');
      expect(persianMessage.other).not.toBe('');
    }
  });

  it('contains no blank or raw-markup message and translates non-invariant copy', () => {
    const english = asCatalog(catalogs.en);
    const persian = asCatalog(catalogs.fa);

    for (const key of Object.keys(english)) {
      const englishParts = messageParts(english[key] ?? '');
      const persianParts = messageParts(persian[key] ?? '');

      for (const part of [...englishParts, ...persianParts]) {
        expect(part.trim()).not.toBe('');
        expect(part).not.toMatch(/<[^>]+>/u);
      }

      if (!invariantKeys.has(key)) {
        expect(persian[key]).not.toEqual(english[key]);
      }
    }
  });

  it('marks an unknown key instead of rendering blank copy', () => {
    const missingKey = 'route.missing.title' as TextMessageKey;
    expect(translateText('en', missingKey)).toBe('[route.missing.title]');
    expect(translateText('fa', missingKey)).toBe('[route.missing.title]');
  });
});
