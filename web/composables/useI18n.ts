import { DEFAULT_LOCALE, LOCALES, MESSAGES, type Locale } from '~/src/locales'

export const LOCALE_STORAGE_KEY = 'xinghai.locale'

type Dictionary = Record<string, unknown>

function lookup(dictionary: Dictionary, path: string[]): unknown {
  let cursor: unknown = dictionary
  for (const segment of path) {
    if (typeof cursor !== 'object' || cursor === null) return undefined
    cursor = (cursor as Dictionary)[segment]
  }
  return cursor
}

function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (match, key: string) =>
    key in params ? String(params[key]) : match)
}

export function useI18n() {
  const locale = useState<Locale>('locale', () => DEFAULT_LOCALE)

  /**
   * Translate a dot-separated key such as `site.heroTitleLine1`.
   * Falls back to the default locale, then to the key itself, so a missing
   * translation is visible in the UI rather than rendering as blank.
   */
  function t(key: string, params?: Record<string, string | number>): string {
    const path = key.split('.')
    const hit = lookup(MESSAGES[locale.value] as Dictionary, path)
      ?? lookup(MESSAGES[DEFAULT_LOCALE] as Dictionary, path)
    return typeof hit === 'string' ? interpolate(hit, params) : key
  }

  function setLocale(next: Locale) {
    locale.value = next
    if (!import.meta.client) return
    localStorage.setItem(LOCALE_STORAGE_KEY, next)
    if (next === 'zh') {
      document.documentElement.lang = 'zh-CN'
    } else if (next === 'zh-Hant') {
      document.documentElement.lang = 'zh-TW'
    } else {
      document.documentElement.lang = 'en'
    }
  }

  function toggleLocale() {
    setLocale(locale.value === 'zh' ? 'en' : locale.value === 'zh-Hant' ? 'zh' : 'zh-Hant')
  }

  function initializeLocale() {
    if (!import.meta.client) return
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY) as Locale | null
    if (stored && stored in MESSAGES) {
      setLocale(stored)
      return
    }
    const navLang = navigator.language.toLowerCase()
    if (navLang.startsWith('zh-hant') || navLang.startsWith('zh-tw')) {
      setLocale('zh-Hant')
    } else if (navLang.startsWith('zh')) {
      setLocale('zh')
    } else {
      setLocale('en')
    }
  }

  return { locale, locales: LOCALES, t, setLocale, toggleLocale, initializeLocale }
}
