/** @import { InterfaceLocaleId } from '../../models/InterfaceLocaleId.js' */
export class LocalePreferences {
  /** @type {readonly InterfaceLocaleId[]} */
  static INTERFACE_LOCALE_IDS = Object.freeze([
    'auto', 'en', 'de', 'es', 'es-419', 'fr', 'ko', 'pl', 'ja', 'zh-CN',
    'zh-TW', 'pt', 'pt-BR', 'gl', 'tr', 'ru', 'cs', 'uk', 'hu', 'he', 'ar',
  ])

  /** @type {'auto'} */
  static DEFAULT_INTERFACE_LOCALE = 'auto'

  /** @type {ReadonlySet<InterfaceLocaleId>} */
  static #INTERFACE_LOCALE_ID_SET = new Set(LocalePreferences.INTERFACE_LOCALE_IDS)

  /** @param {unknown} value @returns {value is InterfaceLocaleId} */
  static isInterfaceLocale(value) {
    return typeof value === 'string' && LocalePreferences.#INTERFACE_LOCALE_ID_SET.has(/** @type {InterfaceLocaleId} */ (value))
  }

  /** @param {unknown} value @returns {InterfaceLocaleId} */
  static normalizeInterfaceLocale(value) {
    return LocalePreferences.isInterfaceLocale(value) ? value : LocalePreferences.DEFAULT_INTERFACE_LOCALE
  }

  /**
   * @param {unknown} value
   * @returns {{ok:true, locale:InterfaceLocaleId}|{ok:false,error:string}}
   */
  static validateInterfaceLocale(value) {
    if (!LocalePreferences.isInterfaceLocale(value)) {
      return { ok: false, error: `interfaceLocale must be one of: ${LocalePreferences.INTERFACE_LOCALE_IDS.join(', ')}` }
    }
    return { ok: true, locale: value }
  }
}
