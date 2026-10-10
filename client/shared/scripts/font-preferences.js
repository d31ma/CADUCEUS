// @ts-check

/** @typedef {'plex-sans'|'system'|'humanist'|'serif'|'monospace'} InterfaceFontId */
/** @typedef {'interface'|InterfaceFontId} ComposerFontId */

export const DEFAULT_INTERFACE_FONT = 'plex-sans'
export const DEFAULT_COMPOSER_FONT = 'interface'

/**
 * Only local/system font stacks are allowed. The catalog deliberately has no
 * URL field and callers persist only the identifier, preventing a preference
 * from becoming a remote request or CSS-injection primitive.
 */
export const INTERFACE_FONT_OPTIONS = Object.freeze([
  Object.freeze({
    id: 'plex-sans',
    label: 'IBM Plex Sans',
    stack: '"IBM Plex Sans", Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
  }),
  Object.freeze({
    id: 'system',
    label: 'System UI',
    stack: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
  }),
  Object.freeze({
    id: 'humanist',
    label: 'Humanist Sans',
    stack: '"Avenir Next", Avenir, Candara, "Segoe UI", sans-serif',
  }),
  Object.freeze({
    id: 'serif',
    label: 'Readable Serif',
    stack: 'Charter, "Bitstream Charter", "Sitka Text", Cambria, Georgia, serif',
  }),
  Object.freeze({
    id: 'monospace',
    label: 'Monospace',
    stack: '"IBM Plex Mono", "SFMono-Regular", Consolas, "Liberation Mono", monospace',
  }),
])

export const COMPOSER_FONT_OPTIONS = Object.freeze([
  Object.freeze({ id: 'interface', label: 'Match interface', stack: 'var(--caduceus-interface-font)' }),
  ...INTERFACE_FONT_OPTIONS,
])

const INTERFACE_BY_ID = new Map(INTERFACE_FONT_OPTIONS.map(option => [option.id, option]))
const COMPOSER_BY_ID = new Map(COMPOSER_FONT_OPTIONS.map(option => [option.id, option]))

export const FONT_SESSION_KEYS = Object.freeze({
  interfaceFont: 'caduceus_interface_font',
  composerFont: 'caduceus_composer_font',
})

/** @param {unknown} value @returns {InterfaceFontId} */
export function normalizeInterfaceFont(value) {
  return typeof value === 'string' && INTERFACE_BY_ID.has(/** @type {InterfaceFontId} */ (value))
    ? /** @type {InterfaceFontId} */ (value)
    : DEFAULT_INTERFACE_FONT
}

/** @param {unknown} value @returns {ComposerFontId} */
export function normalizeComposerFont(value) {
  return typeof value === 'string' && COMPOSER_BY_ID.has(/** @type {ComposerFontId} */ (value))
    ? /** @type {ComposerFontId} */ (value)
    : DEFAULT_COMPOSER_FONT
}

/** @param {unknown} id */
export function interfaceFontStack(id) {
  return INTERFACE_BY_ID.get(normalizeInterfaceFont(id))?.stack || INTERFACE_BY_ID.get(DEFAULT_INTERFACE_FONT)?.stack || 'system-ui, sans-serif'
}

/** @param {unknown} id */
export function composerFontStack(id) {
  return COMPOSER_BY_ID.get(normalizeComposerFont(id))?.stack || 'var(--caduceus-interface-font)'
}

/**
 * Apply catalog values to document-global CSS variables. No caller-controlled
 * string is ever written into a style declaration.
 *
 * @param {{ interfaceFont?: unknown, composerFont?: unknown }} [preferences]
 */
export function applyFontPreferences(preferences = {}) {
  const interfaceFont = normalizeInterfaceFont(preferences.interfaceFont)
  const composerFont = normalizeComposerFont(preferences.composerFont)
  if (typeof document !== 'undefined') {
    document.documentElement.style.setProperty('--caduceus-interface-font', interfaceFontStack(interfaceFont))
    document.documentElement.style.setProperty('--caduceus-composer-font', composerFontStack(composerFont))
  }
  return { interfaceFont, composerFont }
}

/** @param {{ interfaceFont?: unknown, composerFont?: unknown }} preferences */
export function persistFontPreferences(preferences) {
  const normalized = applyFontPreferences(preferences)
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(FONT_SESSION_KEYS.interfaceFont, normalized.interfaceFont)
    sessionStorage.setItem(FONT_SESSION_KEYS.composerFont, normalized.composerFont)
  }
  return normalized
}

export function restoreFontPreferences() {
  let interfaceFont = DEFAULT_INTERFACE_FONT
  let composerFont = DEFAULT_COMPOSER_FONT
  if (typeof sessionStorage !== 'undefined') {
    interfaceFont = normalizeInterfaceFont(sessionStorage.getItem(FONT_SESSION_KEYS.interfaceFont))
    composerFont = normalizeComposerFont(sessionStorage.getItem(FONT_SESSION_KEYS.composerFont))
  }
  return applyFontPreferences({ interfaceFont, composerFont })
}

export function clearFontPreferences() {
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.removeItem(FONT_SESSION_KEYS.interfaceFont)
    sessionStorage.removeItem(FONT_SESSION_KEYS.composerFont)
  }
  return applyFontPreferences()
}
