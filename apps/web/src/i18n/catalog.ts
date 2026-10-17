import appEn from '../app/messages.en.json';
import appFa from '../app/messages.fa.json';
import shellEn from '../app/shell/messages.en.json';
import shellFa from '../app/shell/messages.fa.json';
import authEn from '../features/auth/messages.en.json';
import authFa from '../features/auth/messages.fa.json';
import topologyEn from '../features/product/topology.messages.en.json';
import topologyFa from '../features/product/topology.messages.fa.json';
import trendEn from '../features/product/trend.messages.en.json';
import trendFa from '../features/product/trend.messages.fa.json';
import explorationEn from '../features/product/exploration.messages.en.json';
import explorationFa from '../features/product/exploration.messages.fa.json';
import rulesEn from '../features/product/rules.messages.en.json';
import rulesFa from '../features/product/rules.messages.fa.json';
import usersEn from '../features/users/messages.en.json';
import usersFa from '../features/users/messages.fa.json';
import designSystemEn from '../routes/design-system.messages.en.json';
import designSystemFa from '../routes/design-system.messages.fa.json';
import routesEn from '../routes/messages.en.json';
import routesFa from '../routes/messages.fa.json';
import type { Locale } from './locale';

const enCatalog = {
  ...appEn,
  ...shellEn,
  ...authEn,
  ...topologyEn,
  ...trendEn,
  ...explorationEn,
  ...rulesEn,
  ...usersEn,
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
  ...authFa,
  ...topologyFa,
  ...trendFa,
  ...explorationFa,
  ...rulesFa,
  ...usersFa,
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
