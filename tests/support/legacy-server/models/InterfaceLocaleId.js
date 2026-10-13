/**
 * Interface locales are catalog identifiers, not arbitrary BCP-47 strings.
 * Keeping this boundary closed prevents untrusted values from reaching file
 * paths, dynamic imports, or document attributes.
 */
/** @typedef {'auto'|'en'|'de'|'es'|'es-419'|'fr'|'ko'|'pl'|'ja'|'zh-CN'|'zh-TW'|'pt'|'pt-BR'|'gl'|'tr'|'ru'|'cs'|'uk'|'hu'|'he'|'ar'} InterfaceLocaleId */
export {}
