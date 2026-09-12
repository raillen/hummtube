import { derived, writable, get } from 'svelte/store';
import ptBR from '../../locales/pt-BR.json';
import enUS from '../../locales/en-US.json';

export type SupportedLocale = 'pt-BR' | 'en-US';
type TranslationValue = string | Record<string, unknown>;
type TranslationCatalog = Record<string, TranslationValue>;

const catalogs: Record<SupportedLocale, TranslationCatalog> = {
  'pt-BR': ptBR as TranslationCatalog,
  'en-US': enUS as TranslationCatalog,
};

export const locale = writable<SupportedLocale>('pt-BR');
export const availableLocales: readonly SupportedLocale[] = ['pt-BR', 'en-US'];

function readPath(catalog: TranslationCatalog, key: string): unknown {
  return key.split('.').reduce<unknown>((value, segment) => {
    if (!value || typeof value !== 'object') return undefined;
    return (value as Record<string, unknown>)[segment];
  }, catalog);
}

function interpolate(value: string, variables: Record<string, string | number>): string {
  return value.replace(/\{([\w.-]+)\}/g, (_, name: string) => {
    const replacement = variables[name];
    return replacement === undefined ? `{${name}}` : String(replacement);
  });
}

/** Retorna uma mensagem traduzida com fallback seguro para pt-BR e para a chave. */
export function translate(key: string, variables: Record<string, string | number> = {}): string {
  const selected = get(locale);
  const value = readPath(catalogs[selected], key) ?? readPath(catalogs['pt-BR'], key);
  if (typeof value !== 'string') return key;
  return interpolate(value, variables);
}

/** Alias curto para uso em componentes Svelte e módulos de domínio. */
export const t = translate;

export const currentLocale = derived(locale, ($locale) => $locale);

export function setLocale(next: string): SupportedLocale {
  const normalized: SupportedLocale = next === 'en-US' ? 'en-US' : 'pt-BR';
  locale.set(normalized);
  if (typeof document !== 'undefined') document.documentElement.lang = normalized;
  if (typeof localStorage !== 'undefined') localStorage.setItem('nanotube_locale', normalized);
  return normalized;
}

export function initI18n(): SupportedLocale {
  const saved = typeof localStorage === 'undefined' ? null : localStorage.getItem('nanotube_locale');
  return setLocale(saved || 'pt-BR');
}

export function catalogKeys(selected: SupportedLocale): string[] {
  const keys: string[] = [];
  const visit = (value: TranslationCatalog, prefix = ''): void => {
    for (const [key, child] of Object.entries(value)) {
      const fullKey = prefix ? `${prefix}.${key}` : key;
      if (typeof child === 'string') keys.push(fullKey);
      else visit(child as TranslationCatalog, fullKey);
    }
  };
  visit(catalogs[selected]);
  return keys.sort();
}

export function missingCatalogKeys(base: SupportedLocale, target: SupportedLocale): string[] {
  const targetKeys = new Set(catalogKeys(target));
  return catalogKeys(base).filter((key) => !targetKeys.has(key));
}
