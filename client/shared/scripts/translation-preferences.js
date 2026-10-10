// @ts-check

import { normalizeTranslationLanguage } from './on-device-translation.js'

export const TRANSLATION_PREFERENCES_KEY = 'caduceus-translation-preferences-v1'

/** @typedef {'offer'|'always'|'never'} TranslationPreference */

/** @param {Storage | { getItem: (key: string) => string | null } | undefined} [storage] */
export function loadTranslationPreferences(storage = globalThis.localStorage) {
  try {
    const parsed = JSON.parse(storage?.getItem(TRANSLATION_PREFERENCES_KEY) || '{}')
    const automatic = normalizeLanguageList(parsed?.automatic)
    const disabled = normalizeLanguageList(parsed?.disabled)
    return {
      automatic: automatic.filter(language => !disabled.includes(language)),
      disabled,
    }
  } catch {
    return { automatic: [], disabled: [] }
  }
}

/** @param {string} language @param {Storage | { getItem: (key: string) => string | null } | undefined} [storage] */
export function translationPreferenceFor(language, storage = globalThis.localStorage) {
  const normalized = normalizeTranslationLanguage(language)
  if (!normalized) return 'offer'
  const preferences = loadTranslationPreferences(storage)
  if (preferences.disabled.includes(normalized)) return 'never'
  if (preferences.automatic.includes(normalized)) return 'always'
  return 'offer'
}

/**
 * @param {string} language
 * @param {TranslationPreference} preference
 * @param {Storage | { getItem: (key: string) => string | null, setItem: (key: string, value: string) => void } | undefined} [storage]
 */
export function setTranslationPreference(language, preference, storage = globalThis.localStorage) {
  const normalized = normalizeTranslationLanguage(language)
  if (!normalized || !['offer', 'always', 'never'].includes(preference)) return loadTranslationPreferences(storage)
  const current = loadTranslationPreferences(storage)
  const next = {
    automatic: current.automatic.filter(value => value !== normalized),
    disabled: current.disabled.filter(value => value !== normalized),
  }
  if (preference === 'always') next.automatic.push(normalized)
  if (preference === 'never') next.disabled.push(normalized)
  storage?.setItem(TRANSLATION_PREFERENCES_KEY, JSON.stringify(next))
  globalThis.dispatchEvent?.(new CustomEvent('caduceus:translation-preferences', { detail: next }))
  return next
}

/** @param {Storage | { removeItem: (key: string) => void } | undefined} [storage] */
export function resetTranslationPreferences(storage = globalThis.localStorage) {
  storage?.removeItem(TRANSLATION_PREFERENCES_KEY)
  const next = { automatic: [], disabled: [] }
  globalThis.dispatchEvent?.(new CustomEvent('caduceus:translation-preferences', { detail: next }))
  return next
}

/** @param {unknown} value */
function normalizeLanguageList(value) {
  if (!Array.isArray(value)) return []
  return [...new Set(value.map(normalizeTranslationLanguage).filter(Boolean))]
}
