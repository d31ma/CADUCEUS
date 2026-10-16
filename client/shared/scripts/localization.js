// @ts-check

import ar from '../locales/ar.js'
import en from '../locales/en.js'
import fr from '../locales/fr.js'

export const INTERFACE_LOCALE_STORAGE_KEY = 'caduceus-interface-locale-v1'
/** Native names of the interface locales, in menu order. */
const LOCALE_LABELS = Object.freeze({
  'en': 'English',
  'de': 'Deutsch',
  'es': 'Español',
  'es-419': 'Español (Latinoamérica)',
  'fr': 'Français',
  'ko': '한국어',
  'pl': 'Polski',
  'ja': '日本語',
  'zh-CN': '简体中文',
  'zh-TW': '繁體中文',
  'pt': 'Português',
  'pt-BR': 'Português (Brasil)',
  'gl': 'Galego',
  'tr': 'Türkçe',
  'ru': 'Русский',
  'cs': 'Čeština',
  'uk': 'Українська',
  'hu': 'Magyar',
  'he': 'עברית',
  'ar': 'العربية',
})
/** @type {ReadonlyArray<{id:InterfaceLocaleChoice,label?:string,labelKey?:string}>} */
export const INTERFACE_LOCALE_OPTIONS = Object.freeze([
  { id: 'auto', labelKey: 'locale.automatic' },
  ...Object.entries(LOCALE_LABELS).map(([id, label]) => ({ id: /** @type {keyof typeof LOCALE_LABELS} */ (id), label })),
])

/** @type {ReadonlySet<InterfaceLocaleChoice>} */
const LOCALE_IDS = new Set(INTERFACE_LOCALE_OPTIONS.map(option => option.id))
const CATALOGS = Object.freeze({ en, fr, ar })
/** @type {ReadonlySet<ResolvedInterfaceLocale>} */
const RTL_LOCALES = new Set(['ar', 'he'])
/** @type {Set<string>} */
const INTERFACE_LITERAL_SET = new Set(Object.values(en).filter(value => typeof value === 'string'))
/** @type {Record<string,Record<string,string>>} */
const generatedLiterals = Object.create(null)
/** @type {Promise<void>|null} */
let generatedLiteralPromise = null
/** @type {Map<string,string>} */
const ENGLISH_MESSAGE_IDS = new Map()
for (const [id, value] of Object.entries(en)) {
  if (typeof value === 'string') ENGLISH_MESSAGE_IDS.set(value, id)
}
/** @type {InterfaceLocaleChoice} */
let activeChoice = 'auto'
/** @type {ResolvedInterfaceLocale} */
let activeLocale = 'en'

/** @param {unknown} value @returns {InterfaceLocaleChoice} */
export function normalizeInterfaceLocale(value) {
  return typeof value === 'string' && LOCALE_IDS.has(/** @type {InterfaceLocaleChoice} */ (value))
    ? /** @type {InterfaceLocaleChoice} */ (value)
    : 'auto'
}

/** @param {string} value */
function primaryLanguage(value) {
  try { return new Intl.Locale(value).language.toLowerCase() } catch { return '' }
}

/**
 * Resolve `auto` from the browser's ordered preference list. Only a bundled
 * catalog can win; unsupported languages fall back deterministically to English.
 * @param {unknown} choice
 * @param {readonly string[]} [preferred]
 * @returns {ResolvedInterfaceLocale}
 */
export function resolveInterfaceLocale(choice, preferred = globalThis.navigator?.languages || []) {
  const normalized = normalizeInterfaceLocale(choice)
  if (normalized !== 'auto') return normalized
  for (const candidate of preferred) {
    const requested = String(candidate || '')
    const exact = INTERFACE_LOCALE_OPTIONS.find(option => option.id !== 'auto' && option.id.toLowerCase() === requested.toLowerCase())?.id
    if (exact) return /** @type {ResolvedInterfaceLocale} */ (exact)
    const language = primaryLanguage(requested)
    const match = INTERFACE_LOCALE_OPTIONS.find(option => option.id !== 'auto' && primaryLanguage(option.id) === language)
    if (match) return /** @type {ResolvedInterfaceLocale} */ (match.id)
  }
  return 'en'
}

/**
 * @param {unknown} choice
 * @param {readonly string[]} [preferred]
 * @returns {{choice:InterfaceLocaleChoice, locale:ResolvedInterfaceLocale, direction:'ltr'|'rtl'}}
 */
export function applyInterfaceLocale(choice, preferred) {
  activeChoice = normalizeInterfaceLocale(choice)
  activeLocale = resolveInterfaceLocale(activeChoice, preferred)
  const direction = /** @type {'ltr'|'rtl'} */ (RTL_LOCALES.has(activeLocale) ? 'rtl' : 'ltr')
  const root = globalThis.document?.documentElement
  root?.setAttribute?.('lang', activeLocale)
  root?.setAttribute?.('dir', direction)
  root?.setAttribute?.('data-interface-locale', activeLocale)
  root?.setAttribute?.('data-interface-locale-choice', activeChoice)
  return { choice: activeChoice, locale: activeLocale, direction }
}

/** @param {unknown} choice @returns {{choice:InterfaceLocaleChoice, locale:ResolvedInterfaceLocale, direction:'ltr'|'rtl'}} */
export function persistInterfaceLocale(choice) {
  const normalized = normalizeInterfaceLocale(choice)
  try { globalThis.localStorage?.setItem(INTERFACE_LOCALE_STORAGE_KEY, normalized) } catch {}
  const applied = applyInterfaceLocale(normalized)
  globalThis.dispatchEvent?.(new CustomEvent('caduceus:interface-locale', { detail: applied }))
  return applied
}

export function restoreInterfaceLocale() {
  let stored = 'auto'
  try { stored = globalThis.localStorage?.getItem(INTERFACE_LOCALE_STORAGE_KEY) || 'auto' } catch {}
  return applyInterfaceLocale(stored)
}

export function currentInterfaceLocale() { return activeLocale }
export function currentInterfaceLocaleChoice() { return activeChoice }

/** Load the immutable build-audited interface catalogs exactly once. */
export function loadInterfaceLiteralCatalogs(fetcher = globalThis.fetch?.bind(globalThis)) {
  if (generatedLiteralPromise) return generatedLiteralPromise
  if (typeof fetcher !== 'function') return Promise.reject(new Error('Interface locale asset fetch is unavailable.'))
  generatedLiteralPromise = (async () => {
    const response = await fetcher('/shared/assets/interface-locales.json', {
      credentials: 'same-origin',
      cache: 'force-cache',
    })
    if (!response.ok) throw new Error(`Interface locale asset returned HTTP ${response.status}.`)
    const asset = validateInterfaceLiteralAsset(await response.json())
    for (const source of asset.inventory) INTERFACE_LITERAL_SET.add(source)
    for (const [locale, catalog] of Object.entries(asset.catalogs)) generatedLiterals[locale] = catalog
    globalThis.document?.documentElement?.setAttribute?.('data-interface-catalog-ready', '')
    globalThis.dispatchEvent?.(new CustomEvent('caduceus:interface-catalog-ready'))
  })().catch(error => {
    generatedLiteralPromise = null
    throw error
  })
  return generatedLiteralPromise
}

/** @param {unknown} value */
export function validateInterfaceLiteralAsset(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Interface locale asset must be an object.')
  const asset = /** @type {Record<string,any>} */ (value)
  const inventory = asset.inventory
  if (asset.schemaVersion !== 1 || !Array.isArray(inventory) || inventory.length < 500 || inventory.length > 2_000) {
    throw new Error('Interface locale asset has an invalid schema or inventory size.')
  }
  if (new Set(inventory).size !== inventory.length || inventory.some(source => typeof source !== 'string' || !source.trim() || source.length > 2_000)) {
    throw new Error('Interface locale asset inventory contains invalid or duplicate literals.')
  }
  const expected = INTERFACE_LOCALE_OPTIONS.map(option => option.id).filter(locale => locale !== 'auto' && locale !== 'en').sort()
  const actual = Object.keys(asset.catalogs || {}).sort()
  if (JSON.stringify(actual) !== JSON.stringify(expected)) throw new Error('Interface locale asset has an incomplete locale set.')
  for (const locale of expected) {
    const catalog = asset.catalogs[locale]
    if (!catalog || typeof catalog !== 'object' || Array.isArray(catalog) || Object.keys(catalog).length !== inventory.length) {
      throw new Error(`Interface locale asset has an incomplete ${locale} catalog.`)
    }
    for (const source of inventory) {
      const translated = catalog[source]
      if (typeof translated !== 'string' || !translated.trim() || translated.length > 5_000) {
        throw new Error(`Interface locale asset has an invalid ${locale} translation.`)
      }
    }
  }
  return /** @type {{inventory:string[],catalogs:Record<string,Record<string,string>>}} */ (asset)
}

/** @param {unknown} value */
export function hasInterfaceLiteral(value) {
  return typeof value === 'string' && INTERFACE_LITERAL_SET.has(value.trim())
}

/** Translate only a build-audited interface literal; unknown values pass through. */
export function translateInterfaceLiteral(value, locale = activeLocale) {
  const source = String(value || '')
  if (!INTERFACE_LITERAL_SET.has(source)) return source
  const resolved = resolveInterfaceLocale(locale, [])
  const messageId = ENGLISH_MESSAGE_IDS.get(source)
  const curatedCatalog = /** @type {Record<string,Message>|undefined} */ (CATALOGS[resolved])
  const curated = messageId ? curatedCatalog?.[messageId] : undefined
  if (typeof curated === 'string') return curated
  const generated = generatedLiterals[resolved]?.[source]
  return typeof generated === 'string' && generated.trim() ? generated : source
}

/** @param {string} id @param {Record<string, unknown>} [values] @param {string} [locale] */
export function translate(id, values = {}, locale = activeLocale) {
  const resolved = resolveInterfaceLocale(locale, [])
  const catalog = /** @type {Record<string,Message>} */ (CATALOGS[resolved] || en)
  /** @type {Message | undefined} */
  const entry = catalog[id] ?? en[id]
  if (entry === undefined) return id
  let pattern
  if (typeof entry === 'string') pattern = entry
  else {
    const count = Number(values.count)
    const category = Number.isFinite(count) ? new Intl.PluralRules(resolved).select(count) : 'other'
    pattern = entry[category] || entry.other || Object.values(entry)[0] || id
  }
  pattern = translateInterfaceLiteral(pattern, resolved)
  return pattern.replace(/\{([A-Za-z][A-Za-z0-9_]*)\}/g, (_match, key) => (
    values[key] === undefined || values[key] === null ? '' : String(values[key])
  ))
}

/** @param {number} value @param {Intl.NumberFormatOptions} [options] @param {string} [locale] */
export function formatNumber(value, options = {}, locale = activeLocale) {
  return new Intl.NumberFormat(resolveInterfaceLocale(locale, []), options).format(value)
}

/** @param {string|number|Date|null|undefined} value @param {Intl.DateTimeFormatOptions} [options] @param {string} [locale] */
export function formatDate(value, options = {}, locale = activeLocale) {
  if (value === null || value === undefined || value === '') return ''
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(resolveInterfaceLocale(locale, []), options).format(date)
}
