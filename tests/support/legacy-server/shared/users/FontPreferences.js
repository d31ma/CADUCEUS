export class FontPreferences {
  static DEFAULT_INTERFACE_FONT = 'plex-sans'

  static DEFAULT_COMPOSER_FONT = 'interface'

  static INTERFACE_FONT_IDS = Object.freeze([
    'plex-sans',
    'system',
    'humanist',
    'serif',
    'monospace',
  ])

  static COMPOSER_FONT_IDS = Object.freeze([
    'interface',
    'plex-sans',
    'system',
    'humanist',
    'serif',
    'monospace',
  ])

  static #INTERFACE_FONT_ID_SET = new Set(FontPreferences.INTERFACE_FONT_IDS)

  static #COMPOSER_FONT_ID_SET = new Set(FontPreferences.COMPOSER_FONT_IDS)

  /** @param {unknown} value @returns {value is InterfaceFontId} */
  static isInterfaceFont(value) {
    return typeof value === 'string' && FontPreferences.#INTERFACE_FONT_ID_SET.has(value)
  }

  /** @param {unknown} value @returns {value is ComposerFontId} */
  static isComposerFont(value) {
    return typeof value === 'string' && FontPreferences.#COMPOSER_FONT_ID_SET.has(value)
  }

  /** @param {unknown} value @returns {InterfaceFontId} */
  static normalizeInterfaceFont(value) {
    return FontPreferences.isInterfaceFont(value) ? value : FontPreferences.DEFAULT_INTERFACE_FONT
  }

  /** @param {unknown} value @returns {ComposerFontId} */
  static normalizeComposerFont(value) {
    return FontPreferences.isComposerFont(value) ? value : FontPreferences.DEFAULT_COMPOSER_FONT
  }
}
