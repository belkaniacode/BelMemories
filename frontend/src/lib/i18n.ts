// UI language: Russian when the OS language is Russian, English otherwise.
// It is resolved once before the app mounts (see main.ts) and never changes,
// so plain functions are enough — no reactive store is needed.

export type Lang = 'ru' | 'en'

let lang: Lang = 'en'

/** Maps a locale such as "ru_RU.UTF-8" or "ru-RU" to a supported language. */
export function langFromLocale(locale: string | null | undefined): Lang {
  return /^ru([_.-]|$)/i.test((locale ?? '').trim()) ? 'ru' : 'en'
}

export function setLang(l: Lang): void {
  lang = l
  document.documentElement.lang = l
}

export function getLang(): Lang {
  return lang
}

/** Returns the Russian or English text for the current language. */
export function tr(ru: string, en: string): string {
  return lang === 'ru' ? ru : en
}

/**
 * Plural form: ru = [one, few, many] ("файл", "файла", "файлов"),
 * en = [one, other] ("file", "files"). Returns only the word.
 */
export function plural(n: number, ru: [string, string, string], en: [string, string]): string {
  if (lang !== 'ru') return n === 1 ? en[0] : en[1]
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return ru[0]
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return ru[1]
  return ru[2]
}

/** Locale for number/date formatting. */
export function numberLocale(): string {
  return lang === 'ru' ? 'ru-RU' : 'en-US'
}
