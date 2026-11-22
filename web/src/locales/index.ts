import zh from './zh'

export type Locale = 'zh' | 'zh-Hant' | 'en'

export const LOCALES: { value: Locale; label: string; short: string }[] = [
  { value: 'zh', label: '简体中文', short: '中' },
  { value: 'zh-Hant', label: '繁體中文', short: '繁' },
  { value: 'en', label: 'English', short: 'EN' },
]

export const DEFAULT_LOCALE = 'zh'

export const defaultMessages = zh
export const messages: Partial<Record<Locale, Record<string, unknown>>> = { zh }

export async function loadMessages(locale: Locale) {
  if (locale === 'zh' || messages[locale]) return
  const dictionary = locale === 'en' ? await import('./en') : await import('./zh-Hant')
  messages[locale] = dictionary.default
}
