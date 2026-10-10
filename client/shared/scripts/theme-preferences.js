// @ts-check

/** @typedef {'automatic'|'light'|'dark'|'darkside'|'less-is-more'|'taiga'|'ubuntu'} ActiveThemeId */
/** @typedef {'light'|'less-is-more'|'taiga'|'ubuntu'} LightThemeId */
/** @typedef {'dark'|'darkside'} DarkThemeId */

export const THEME_OPTIONS = Object.freeze([
  Object.freeze({ id: 'automatic', label: 'Automatic', description: 'Follow this device’s light or dark appearance.', baseMode: 'automatic', colors: ['#f8faf9', '#1f6f8b', '#1c2224'] }),
  Object.freeze({ id: 'light', label: 'Light', description: 'CADUCEUS’s balanced light interface.', baseMode: 'light', colors: ['#f8faf9', '#eef3f1', '#1f6f8b'] }),
  Object.freeze({ id: 'dark', label: 'Dark', description: 'CADUCEUS’s low-glare dark interface.', baseMode: 'dark', colors: ['#1c2224', '#161c1e', '#83cde3'] }),
  Object.freeze({ id: 'darkside', label: 'Darkside', description: 'A deep aubergine sidebar with a warm accent.', baseMode: 'dark', colors: ['#242230', '#313042', '#f18260'] }),
  Object.freeze({ id: 'less-is-more', label: 'Less Is More', description: 'A quiet, compact, low-chrome workspace.', baseMode: 'light', colors: ['#ffffff', '#f7f7f5', '#606b73'] }),
  Object.freeze({ id: 'taiga', label: 'Taiga', description: 'A clean, cool, mailbox-inspired workspace.', baseMode: 'light', colors: ['#f5f8fa', '#e8eef2', '#4f7c8f'] }),
  Object.freeze({ id: 'ubuntu', label: 'Ubuntu', description: 'Aubergine navigation with Ubuntu orange.', baseMode: 'light', colors: ['#f7f4f2', '#4b243f', '#e95420'] }),
])

export const LIGHT_THEME_OPTIONS = Object.freeze([THEME_OPTIONS[1], THEME_OPTIONS[4], THEME_OPTIONS[5], THEME_OPTIONS[6]])
export const DARK_THEME_OPTIONS = Object.freeze([THEME_OPTIONS[2], THEME_OPTIONS[3]])

export const DEFAULT_THEME_PREFERENCES = Object.freeze({
  activeTheme: /** @type {ActiveThemeId} */ ('automatic'),
  lightTheme: /** @type {LightThemeId} */ ('light'),
  darkTheme: /** @type {DarkThemeId} */ ('dark'),
})

export const THEME_STORAGE_KEY = 'caduceus-theme-preferences'

const BY_ID = new Map(THEME_OPTIONS.map(theme => [theme.id, theme]))
const LIGHT_IDS = new Set(LIGHT_THEME_OPTIONS.map(theme => theme.id))
const DARK_IDS = new Set(DARK_THEME_OPTIONS.map(theme => theme.id))

/** @param {unknown} value @returns {value is ActiveThemeId} */
export function isActiveTheme(value) {
  return typeof value === 'string' && BY_ID.has(/** @type {ActiveThemeId} */ (value))
}

/** @param {unknown} value @returns {value is LightThemeId} */
export function isLightTheme(value) {
  return typeof value === 'string' && LIGHT_IDS.has(/** @type {LightThemeId} */ (value))
}

/** @param {unknown} value @returns {value is DarkThemeId} */
export function isDarkTheme(value) {
  return typeof value === 'string' && DARK_IDS.has(/** @type {DarkThemeId} */ (value))
}

/** @param {unknown} value */
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

/** @param {unknown} preferences @param {boolean} [prefersDark] */
export function resolveTheme(preferences, prefersDark = systemPrefersDark()) {
  const normalized = normalizeThemePreferences(preferences)
  return normalized.activeTheme === 'automatic'
    ? (prefersDark ? normalized.darkTheme : normalized.lightTheme)
    : normalized.activeTheme
}

/** @param {unknown} theme */
export function themeBaseMode(theme) {
  const option = BY_ID.get(isActiveTheme(theme) ? theme : 'light')
  return option?.baseMode === 'dark' ? 'dark' : 'light'
}

export function systemPrefersDark() {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches
}

/** @param {unknown} preferences */
export function applyThemePreferences(preferences) {
  const normalized = normalizeThemePreferences(preferences)
  const resolvedTheme = resolveTheme(normalized)
  const baseMode = themeBaseMode(resolvedTheme)
  if (typeof document !== 'undefined') {
    const root = document.documentElement
    root.setAttribute('w-theme', baseMode)
    root.setAttribute('data-theme', baseMode)
    root.setAttribute('data-ui-theme', resolvedTheme)
    root.setAttribute('data-theme-choice', normalized.activeTheme)
  }
  return { ...normalized, resolvedTheme, baseMode }
}

/** @param {unknown} preferences */
export function persistThemePreferences(preferences) {
  const normalized = normalizeThemePreferences(preferences)
  try { localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify(normalized)) } catch {}
  return applyThemePreferences(normalized)
}

export function restoreThemePreferences() {
  /** @type {unknown} */
  let stored = null
  try { stored = JSON.parse(localStorage.getItem(THEME_STORAGE_KEY) || 'null') } catch {}
  if (!stored) {
    // One-time migration from the previous single-value preference.
    /** @type {string | null} */
    let legacy = null
    try { legacy = localStorage.getItem('caduceus-theme') } catch {}
    if (legacy === 'light' || legacy === 'dark' || legacy === 'auto') {
      stored = { ...DEFAULT_THEME_PREFERENCES, activeTheme: legacy === 'auto' ? 'automatic' : legacy }
    }
  }
  return persistThemePreferences(stored)
}

/** @param {() => void} callback */
export function watchSystemTheme(callback) {
  if (typeof matchMedia !== 'function') return () => {}
  const query = matchMedia('(prefers-color-scheme: dark)')
  const handler = () => callback()
  query.addEventListener?.('change', handler)
  return () => query.removeEventListener?.('change', handler)
}
