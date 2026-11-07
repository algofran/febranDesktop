import { ref } from 'vue'
import { LOCALES, messages, type Dict, type Locale } from './locales'

const STORAGE_KEY = 'febran-desktop-locale'

function initialLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'en' || saved === 'id') return saved
  return navigator.language?.toLowerCase().startsWith('id') ? 'id' : 'en'
}

const locale = ref<Locale>(initialLocale())

function lookup(dict: Dict, path: string): string | undefined {
  let cur: unknown = dict
  for (const part of path.split('.')) {
    if (cur && typeof cur === 'object' && part in (cur as Dict)) cur = (cur as Dict)[part]
    else return undefined
  }
  return typeof cur === 'string' ? cur : undefined
}

/** Translate "<screen>.<key>"; "{name}" placeholders are filled from params. Reading locale makes templates reactive. */
export function t(key: string, params?: Record<string, string | number>): string {
  const msg = lookup(messages[locale.value], key) ?? lookup(messages.en, key) ?? key
  return params ? msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`)) : msg
}

export function setLocale(l: Locale) {
  locale.value = l
  localStorage.setItem(STORAGE_KEY, l)
  document.documentElement.lang = l
}
document.documentElement.lang = locale.value

export function useI18n() {
  return { t, locale, setLocale, locales: LOCALES }
}
