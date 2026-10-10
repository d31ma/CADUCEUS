/**
 * Font preferences cross an untrusted HTTP boundary, so CADUCEUS stores only
 * catalog identifiers. CSS font-family text never reaches FYLO or the DOM.
 */
/** @typedef {'plex-sans'|'system'|'humanist'|'serif'|'monospace'} InterfaceFontId */
/** @typedef {'interface'|InterfaceFontId} ComposerFontId */
export const DEFAULT_INTERFACE_FONT = 'plex-sans'
export const DEFAULT_COMPOSER_FONT = 'interface'

export const INTERFACE_FONT_IDS = Object.freeze([
  'plex-sans',
  'system',
  'humanist',
  'serif',
  'monospace',
])

export const COMPOSER_FONT_IDS = Object.freeze([
  'interface',
  'plex-sans',
  'system',
  'humanist',
  'serif',
  'monospace',
])

const INTERFACE_FONT_ID_SET = new Set(INTERFACE_FONT_IDS)
const COMPOSER_FONT_ID_SET = new Set(COMPOSER_FONT_IDS)

/** @param {unknown} value @returns {value is InterfaceFontId} */
export function isInterfaceFont(value) {
  return typeof value === 'string' && INTERFACE_FONT_ID_SET.has(value)
}

/** @param {unknown} value @returns {value is ComposerFontId} */
export function isComposerFont(value) {
  return typeof value === 'string' && COMPOSER_FONT_ID_SET.has(value)
}

/** @param {unknown} value @returns {InterfaceFontId} */
export function normalizeInterfaceFont(value) {
  return isInterfaceFont(value) ? value : DEFAULT_INTERFACE_FONT
}

/** @param {unknown} value @returns {ComposerFontId} */
export function normalizeComposerFont(value) {
  return isComposerFont(value) ? value : DEFAULT_COMPOSER_FONT
}
