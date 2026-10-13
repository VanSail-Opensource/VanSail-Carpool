import { computed, readonly, shallowRef } from 'vue'

import { localeMessages, type Locale, type LocaleKey } from './generated'
import { isBrowserExperience } from '@/utils/experience'

const storageKey = 'tx-carpool-locale'
const supportedLocales = Object.keys(localeMessages) as Locale[]

function storedLocale(): string | null {
  try {
    return globalThis.localStorage?.getItem(storageKey) ?? null
  } catch {
    return null
  }
}

function detectLocale(): Locale {
  const stored = storedLocale()
  if (stored && supportedLocales.includes(stored as Locale)) return stored as Locale
  const telegramLanguage = !isBrowserExperience() ? window.Telegram?.WebApp?.initDataUnsafe?.user?.language_code?.toLowerCase() : undefined
  if (telegramLanguage?.startsWith('zh')) return 'zh-CN'
  if (isBrowserExperience() && navigator.language.toLowerCase().startsWith('zh')) return 'zh-CN'
  return 'en'
}

const locale = shallowRef<Locale>(detectLocale())

function applyDocumentLocale(next: Locale): void {
  if (typeof document === 'undefined') return
  document.documentElement.lang = next
  document.title = t('app.name')
}

applyDocumentLocale(locale.value)

function lookup(key: string, source: unknown): unknown {
  const segments = key.split('.')
  let value = source
  for (let index = 0; index < segments.length; index++) {
    if (!value || typeof value !== 'object') return undefined
    const record = value as Record<string, unknown>
    const remaining = segments.slice(index).join('.')
    if (Object.hasOwn(record, remaining)) return record[remaining]
    value = record[segments[index]!]
  }
  return value
}

export function setLocale(next: Locale): void {
  if (!supportedLocales.includes(next)) return
  locale.value = next
  try {
    globalThis.localStorage?.setItem(storageKey, next)
  } catch {
    // Locale choice remains available for this session in restricted WebViews.
  }
  applyDocumentLocale(next)
}

export function getLocale(): Locale {
  return locale.value
}

export function t(key: LocaleKey | string, variables: Record<string, string | number> = {}): string {
  const value = lookup(key, localeMessages[locale.value]) ?? lookup(key, localeMessages.en)
  if (typeof value !== 'string') return key
  return value.replace(/\{(\w+)\}/g, (_, name: string) => String(variables[name] ?? `{${name}}`))
}

export function localizedError(caught: unknown, fallbackKey = 'common.error'): string {
  if (caught && typeof caught === 'object' && 'code' in caught) {
    const code = String((caught as { code?: unknown }).code ?? '')
    const message = t(`errors.api.${code}`)
    if (message !== `errors.api.${code}`) return message
  }
  return t(fallbackKey)
}

export function useI18n() {
  return {
    locale: readonly(locale),
    locales: computed(() => supportedLocales),
    t,
    setLocale,
  }
}
