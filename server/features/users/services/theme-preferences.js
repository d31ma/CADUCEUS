/**
 * Theme preferences cross the untrusted HTTP boundary as catalog identifiers.
 * CSS, file paths, and package names supplied by callers are never persisted.
 */
export const ACTIVE_THEME_IDS = Object.freeze([
  'automatic',
  'light',
  'dark',
  'darkside',
  'less-is-more',
  'taiga',
  'ubuntu',
])

export const LIGHT_THEME_IDS = Object.freeze(['light', 'less-is-more', 'taiga', 'ubuntu'])
export const DARK_THEME_IDS = Object.freeze(['dark', 'darkside'])

export const DEFAULT_THEME_PREFERENCES = Object.freeze({
  activeTheme: 'automatic',
  lightTheme: 'light',
  darkTheme: 'dark',
})

const ACTIVE_THEME_ID_SET = new Set(ACTIVE_THEME_IDS)
const LIGHT_THEME_ID_SET = new Set(LIGHT_THEME_IDS)
const DARK_THEME_ID_SET = new Set(DARK_THEME_IDS)

/** @typedef {'automatic'|'light'|'dark'|'darkside'|'less-is-more'|'taiga'|'ubuntu'} ActiveThemeId */
/** @typedef {'light'|'less-is-more'|'taiga'|'ubuntu'} LightThemeId */
/** @typedef {'dark'|'darkside'} DarkThemeId */

/** @param {unknown} value @returns {value is ActiveThemeId} */
export function isActiveTheme(value) {
  return typeof value === 'string' && ACTIVE_THEME_ID_SET.has(value)
}

/** @param {unknown} value @returns {value is LightThemeId} */
export function isLightTheme(value) {
  return typeof value === 'string' && LIGHT_THEME_ID_SET.has(value)
}

/** @param {unknown} value @returns {value is DarkThemeId} */
export function isDarkTheme(value) {
  return typeof value === 'string' && DARK_THEME_ID_SET.has(value)
}

/** @param {unknown} value @returns {{ activeTheme: ActiveThemeId, lightTheme: LightThemeId, darkTheme: DarkThemeId }} */
export function normalizeThemePreferences(value) {
  const input = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, unknown>} */ (value)
    : {}
  return {
    activeTheme: isActiveTheme(input.activeTheme) ? input.activeTheme : DEFAULT_THEME_PREFERENCES.activeTheme,
    lightTheme: isLightTheme(input.lightTheme) ? input.lightTheme : DEFAULT_THEME_PREFERENCES.lightTheme,
    darkTheme: isDarkTheme(input.darkTheme) ? input.darkTheme : DEFAULT_THEME_PREFERENCES.darkTheme,
  }
}

/**
 * @param {unknown} value
 * @returns {{ ok: true, preferences: Record<string, string> } | { ok: false, error: string }}
 */
export function validateThemePreferences(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'themePreferences must be an object' }
  }
  const input = /** @type {Record<string, unknown>} */ (value)
  const unknown = Object.keys(input).filter(key => !['activeTheme', 'lightTheme', 'darkTheme'].includes(key))
  if (unknown.length > 0) return { ok: false, error: `Unknown theme preference: ${unknown[0]}` }
  if (input.activeTheme !== undefined && !isActiveTheme(input.activeTheme)) {
    return { ok: false, error: 'activeTheme must be one of: automatic, light, dark, darkside, less-is-more, taiga, ubuntu' }
  }
  if (input.lightTheme !== undefined && !isLightTheme(input.lightTheme)) {
    return { ok: false, error: 'lightTheme must be one of: light, less-is-more, taiga, ubuntu' }
  }
  if (input.darkTheme !== undefined && !isDarkTheme(input.darkTheme)) {
    return { ok: false, error: 'darkTheme must be one of: dark, darkside' }
  }
  return { ok: true, preferences: normalizeThemePreferences(input) }
}
