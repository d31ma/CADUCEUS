/** @import { ActiveThemeId } from '../../models/ActiveThemeId.js' */
/** @import { LightThemeId } from '../../models/LightThemeId.js' */
/** @import { DarkThemeId } from '../../models/DarkThemeId.js' */
export class ThemePreferences {
  /**
   * Theme preferences cross the untrusted HTTP boundary as catalog identifiers.
   * CSS, file paths, and package names supplied by callers are never persisted.
   */
  static ACTIVE_THEME_IDS = Object.freeze([
    'automatic',
    'light',
    'dark',
    'darkside',
    'less-is-more',
    'taiga',
    'ubuntu',
  ])

  static LIGHT_THEME_IDS = Object.freeze(['light', 'less-is-more', 'taiga', 'ubuntu'])

  static DARK_THEME_IDS = Object.freeze(['dark', 'darkside'])

  static DEFAULT_THEME_PREFERENCES = Object.freeze({
    activeTheme: 'automatic',
    lightTheme: 'light',
    darkTheme: 'dark',
  })

  static #ACTIVE_THEME_ID_SET = new Set(ThemePreferences.ACTIVE_THEME_IDS)

  static #LIGHT_THEME_ID_SET = new Set(ThemePreferences.LIGHT_THEME_IDS)

  static #DARK_THEME_ID_SET = new Set(ThemePreferences.DARK_THEME_IDS)

  /** @param {unknown} value @returns {value is ActiveThemeId} */
  static isActiveTheme(value) {
    return typeof value === 'string' && ThemePreferences.#ACTIVE_THEME_ID_SET.has(value)
  }

  /** @param {unknown} value @returns {value is LightThemeId} */
  static isLightTheme(value) {
    return typeof value === 'string' && ThemePreferences.#LIGHT_THEME_ID_SET.has(value)
  }

  /** @param {unknown} value @returns {value is DarkThemeId} */
  static isDarkTheme(value) {
    return typeof value === 'string' && ThemePreferences.#DARK_THEME_ID_SET.has(value)
  }

  /** @param {unknown} value @returns {{ activeTheme: ActiveThemeId, lightTheme: LightThemeId, darkTheme: DarkThemeId }} */
  static normalizeThemePreferences(value) {
    const input = value && typeof value === 'object' && !Array.isArray(value)
      ? /** @type {Record<string, unknown>} */ (value)
      : {}
    return {
      activeTheme: ThemePreferences.isActiveTheme(input.activeTheme) ? input.activeTheme : ThemePreferences.DEFAULT_THEME_PREFERENCES.activeTheme,
      lightTheme: ThemePreferences.isLightTheme(input.lightTheme) ? input.lightTheme : ThemePreferences.DEFAULT_THEME_PREFERENCES.lightTheme,
      darkTheme: ThemePreferences.isDarkTheme(input.darkTheme) ? input.darkTheme : ThemePreferences.DEFAULT_THEME_PREFERENCES.darkTheme,
    }
  }

  /**
   * @param {unknown} value
   * @returns {{ ok: true, preferences: Record<string, string> } | { ok: false, error: string }}
   */
  static validateThemePreferences(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return { ok: false, error: 'themePreferences must be an object' }
    }
    const input = /** @type {Record<string, unknown>} */ (value)
    const unknown = Object.keys(input).filter(key => !['activeTheme', 'lightTheme', 'darkTheme'].includes(key))
    if (unknown.length > 0) return { ok: false, error: `Unknown theme preference: ${unknown[0]}` }
    if (input.activeTheme !== undefined && !ThemePreferences.isActiveTheme(input.activeTheme)) {
      return { ok: false, error: 'activeTheme must be one of: automatic, light, dark, darkside, less-is-more, taiga, ubuntu' }
    }
    if (input.lightTheme !== undefined && !ThemePreferences.isLightTheme(input.lightTheme)) {
      return { ok: false, error: 'lightTheme must be one of: light, less-is-more, taiga, ubuntu' }
    }
    if (input.darkTheme !== undefined && !ThemePreferences.isDarkTheme(input.darkTheme)) {
      return { ok: false, error: 'darkTheme must be one of: dark, darkside' }
    }
    return { ok: true, preferences: ThemePreferences.normalizeThemePreferences(input) }
  }
}
