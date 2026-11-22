import { DEFAULT_LOCALE, LOCALES, defaultMessages, loadMessages, messages, type Locale } from '~/src/locales'

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
  const requestedLocale = useState<Locale>('requested-locale', () => locale.value)

  /**
   * Translate a dot-separated key such as `site.heroTitleLine1`.
   * Falls back to the default locale, then to the key itself, so a missing
   * translation is visible in the UI rather than rendering as blank.
   */
  function t(key: string, params?: Record<string, string | number>): string {
    const path = key.split('.')
    const hit = lookup(messages[locale.value] ?? defaultMessages, path)
      ?? lookup(defaultMessages, path)
    return typeof hit === 'string' ? interpolate(hit, params) : key
  }

  async function setLocale(next: Locale) {
    requestedLocale.value = next
    if (!messages[next]) {
      try {
        await loadMessages(next)
      } catch {
        return
      }
      if (requestedLocale.value !== next) return
    }
    locale.value = next
    if (!import.meta.client) return
    if (next === 'zh') {
      document.documentElement.lang = 'zh-CN'
    } else if (next === 'zh-Hant') {
      document.documentElement.lang = 'zh-TW'
    } else {
      document.documentElement.lang = 'en'
    }
    try { localStorage.setItem(LOCALE_STORAGE_KEY, next) } catch { return }
  }

  function toggleLocale() {
    setLocale(locale.value === 'zh' ? 'en' : locale.value === 'zh-Hant' ? 'zh' : 'zh-Hant')
  }

  function initializeLocale() {
    if (!import.meta.client) return
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY)
    const saved = LOCALES.find(item => item.value === stored)
    if (saved) {
      setLocale(saved.value)
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
