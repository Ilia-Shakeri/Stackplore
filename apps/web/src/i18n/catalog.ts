import appEn from '../app/messages.en.json';
import appFa from '../app/messages.fa.json';
import shellEn from '../app/shell/messages.en.json';
import shellFa from '../app/shell/messages.fa.json';
import designSystemEn from '../routes/design-system.messages.en.json';
import designSystemFa from '../routes/design-system.messages.fa.json';
import routesEn from '../routes/messages.en.json';
import routesFa from '../routes/messages.fa.json';
import type { Locale } from './locale';

const enCatalog = {
  ...appEn,
  ...shellEn,
  ...routesEn,
  ...designSystemEn,
} as const;

export type MessageKey = keyof typeof enCatalog;
export type PluralCategory = Intl.LDMLPluralRule;
export type PluralMessage = Readonly<Partial<Record<PluralCategory, string>> & { other: string }>;
export type CatalogMessage = string | PluralMessage;

export type TextMessageKey = {
  [Key in MessageKey]: (typeof enCatalog)[Key] extends string ? Key : never;
}[MessageKey];

export type PluralMessageKey = Exclude<MessageKey, TextMessageKey>;

const faCatalog = {
  ...appFa,
  ...shellFa,
  ...routesFa,
  ...designSystemFa,
} as const;

export const catalogs = {
  en: enCatalog,
  fa: faCatalog,
} as const satisfies Record<Locale, Record<MessageKey, CatalogMessage>>;

export function findMessage(locale: Locale, key: string): CatalogMessage | undefined {
  const selectedCatalog = catalogs[locale] as Readonly<Record<string, CatalogMessage>>;
  const fallbackCatalog = catalogs.en as Readonly<Record<string, CatalogMessage>>;
  return selectedCatalog[key] ?? fallbackCatalog[key];
}
