/**
 * Interface locales are catalog identifiers, not arbitrary BCP-47 strings.
 * Keeping this boundary closed prevents untrusted values from reaching file
 * paths, dynamic imports, or document attributes.
 */
/** @typedef {'auto'|'en'|'de'|'es'|'es-419'|'fr'|'ko'|'pl'|'ja'|'zh-CN'|'zh-TW'|'pt'|'pt-BR'|'gl'|'tr'|'ru'|'cs'|'uk'|'hu'|'he'|'ar'} InterfaceLocaleId */
/** @type {readonly InterfaceLocaleId[]} */
export const INTERFACE_LOCALE_IDS = Object.freeze([
  'auto', 'en', 'de', 'es', 'es-419', 'fr', 'ko', 'pl', 'ja', 'zh-CN',
  'zh-TW', 'pt', 'pt-BR', 'gl', 'tr', 'ru', 'cs', 'uk', 'hu', 'he', 'ar',
])
export const DEFAULT_INTERFACE_LOCALE = 'auto'

/** @type {ReadonlySet<InterfaceLocaleId>} */
const INTERFACE_LOCALE_ID_SET = new Set(INTERFACE_LOCALE_IDS)

/** @param {unknown} value @returns {value is InterfaceLocaleId} */
export function isInterfaceLocale(value) {
  return typeof value === 'string' && INTERFACE_LOCALE_ID_SET.has(/** @type {InterfaceLocaleId} */ (value))
}

/** @param {unknown} value @returns {InterfaceLocaleId} */
export function normalizeInterfaceLocale(value) {
  return isInterfaceLocale(value) ? value : DEFAULT_INTERFACE_LOCALE
}

/**
 * @param {unknown} value
 * @returns {{ok:true, locale:InterfaceLocaleId}|{ok:false,error:string}}
 */
export function validateInterfaceLocale(value) {
  if (!isInterfaceLocale(value)) {
    return { ok: false, error: `interfaceLocale must be one of: ${INTERFACE_LOCALE_IDS.join(', ')}` }
  }
  return { ok: true, locale: value }
}
