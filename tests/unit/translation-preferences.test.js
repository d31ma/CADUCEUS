import { describe, expect, test } from 'bun:test'
import { TranslationPreferencesUtils } from '../../client/shared/utils/TranslationPreferencesUtils.js'

const loadTranslationPreferences = (...args) => TranslationPreferencesUtils.loadTranslationPreferences(...args)
const resetTranslationPreferences = (...args) => TranslationPreferencesUtils.resetTranslationPreferences(...args)
const setTranslationPreference = (...args) => TranslationPreferencesUtils.setTranslationPreference(...args)
const translationPreferenceFor = (...args) => TranslationPreferencesUtils.translationPreferenceFor(...args)
const TRANSLATION_PREFERENCES_KEY = TranslationPreferencesUtils.translationPreferencesKey()

function memoryStorage(initial = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: key => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: key => values.delete(key),
  }
}

describe('translation preferences', () => {
  test('normalizes corrupt, duplicate, and conflicting language lists', () => {
    const storage = memoryStorage({
      [TRANSLATION_PREFERENCES_KEY]: JSON.stringify({
        automatic: ['ES', 'fr-CA', 'es', '../en'],
        disabled: ['fr', 'xx'],
      }),
    })
    expect(loadTranslationPreferences(storage)).toEqual({ automatic: ['es'], disabled: ['fr'] })
    expect(translationPreferenceFor('es-MX', storage)).toBe('always')
    expect(translationPreferenceFor('fr', storage)).toBe('never')
    expect(translationPreferenceFor('de', storage)).toBe('offer')
  })

  test('moves one language atomically between offer, always, and never', () => {
    const storage = memoryStorage()
    expect(setTranslationPreference('es', 'always', storage)).toEqual({ automatic: ['es'], disabled: [] })
    expect(setTranslationPreference('es', 'never', storage)).toEqual({ automatic: [], disabled: ['es'] })
    expect(setTranslationPreference('es', 'offer', storage)).toEqual({ automatic: [], disabled: [] })
  })

  test('resets all local translation choices', () => {
    const storage = memoryStorage()
    setTranslationPreference('ja', 'always', storage)
    expect(resetTranslationPreferences(storage)).toEqual({ automatic: [], disabled: [] })
    expect(loadTranslationPreferences(storage)).toEqual({ automatic: [], disabled: [] })
  })
})
