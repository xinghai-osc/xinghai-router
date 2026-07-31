import en from './en'
import zh from './zh'
import zhHant from './zh-Hant'

export const MESSAGES = { zh, 'zh-Hant': zhHant, en }

export type Locale = keyof typeof MESSAGES

export const LOCALES: { value: Locale; label: string; short: string }[] = [
  { value: 'zh', label: '简体中文', short: '中' },
  { value: 'zh-Hant', label: '繁體中文', short: '繁' },
  { value: 'en', label: 'English', short: 'EN' },
]

export const DEFAULT_LOCALE: Locale = 'zh'
