import { afterEach, describe, expect, it } from 'bun:test'
import {
  ACTIVE_THEME_IDS,
  DARK_THEME_IDS,
  LIGHT_THEME_IDS,
  normalizeThemePreferences as normalizeServer,
  validateThemePreferences,
} from '../../server/shared/users/theme-preferences.js'
import {
  DARK_THEME_OPTIONS,
  LIGHT_THEME_OPTIONS,
  THEME_OPTIONS,
  applyThemePreferences,
  normalizeThemePreferences,
  resolveTheme,
} from '../../client/shared/scripts/theme-preferences.js'

const originalDocument = globalThis.document
const originalMatchMedia = globalThis.matchMedia

afterEach(() => {
  globalThis.document = originalDocument
  globalThis.matchMedia = originalMatchMedia
})

describe('theme preferences', () => {
  it('keeps browser and persistence catalogs in lockstep', () => {
    expect(THEME_OPTIONS.map(theme => theme.id)).toEqual([...ACTIVE_THEME_IDS])
    expect(LIGHT_THEME_OPTIONS.map(theme => theme.id)).toEqual([...LIGHT_THEME_IDS])
    expect(DARK_THEME_OPTIONS.map(theme => theme.id)).toEqual([...DARK_THEME_IDS])
  })

  it('normalizes hostile values and rejects caller-defined theme fields', () => {
    const hostile = {
      activeTheme: 'url(https://themes.example.test/steal.css)',
      lightTheme: 'light; background:red',
      darkTheme: '../../../theme.css',
    }
    expect(normalizeThemePreferences(hostile)).toEqual(normalizeServer(hostile))
    expect(normalizeThemePreferences(hostile)).toEqual({
      activeTheme: 'automatic', lightTheme: 'light', darkTheme: 'dark',
    })
    expect(validateThemePreferences({ activeTheme: 'light', stylesheet: 'https://evil.test/x.css' })).toEqual({
      ok: false, error: 'Unknown theme preference: stylesheet',
    })
  })

  it('resolves automatic light and dark slots and applies only fixed attributes', () => {
    const attrs = new Map()
    globalThis.document = {
      documentElement: { setAttribute: (name, value) => attrs.set(name, value) },
    }
    globalThis.matchMedia = () => ({ matches: true })
    const preferences = { activeTheme: 'automatic', lightTheme: 'taiga', darkTheme: 'darkside' }
    expect(resolveTheme(preferences, false)).toBe('taiga')
    expect(resolveTheme(preferences, true)).toBe('darkside')
    expect(applyThemePreferences(preferences)).toEqual({
      ...preferences, resolvedTheme: 'darkside', baseMode: 'dark',
    })
    expect(Object.fromEntries(attrs)).toEqual({
      'w-theme': 'dark',
      'data-theme': 'dark',
      'data-ui-theme': 'darkside',
      'data-theme-choice': 'automatic',
    })
  })
})
