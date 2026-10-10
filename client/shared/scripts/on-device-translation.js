// @ts-check

import { formatNumber } from './localization.js'

import { francAll } from './translation-detection-vendor.js'
import {
  createOfflineTranslator,
  offlineTranslationSupported,
} from './offline-translation-engine.js'
import {
  FRANC_TO_TRANSLATION_LANGUAGE,
  TRANSLATION_LANGUAGES,
} from './translation-languages.js'

export { TRANSLATION_LANGUAGES } from './translation-languages.js'

export const MAX_TRANSLATION_CHARACTERS = 20_000

const TRANSLATION_TO_FRANC_LANGUAGE = Object.freeze(Object.fromEntries(
  Object.entries(FRANC_TO_TRANSLATION_LANGUAGE).map(([franc, translation]) => [translation, franc]),
))

const TRANSLATED_SNIPPETS_LIMIT = 1_000
/** @type {Map<string, string>} */
const translatedSnippets = new Map()

/** @param {unknown} value */
export function normalizeTranslationLanguage(value) {
  const primary = String(value || '').trim().toLowerCase().split('-')[0]
  return Object.hasOwn(TRANSLATION_LANGUAGES, primary) ? primary : ''
}

/** Resolve the user's preferred target without ever returning an unsupported tag. */
export function preferredTranslationLanguage(locale = globalThis.navigator?.language || 'en') {
  return normalizeTranslationLanguage(locale) || 'en'
}

/** @returns {any} */
function translatorApi() { return /** @type {any} */ (globalThis).Translator }

/** @returns {any} */
function detectorApi() { return /** @type {any} */ (globalThis).LanguageDetector }

export function onDeviceTranslationSupported() {
  return typeof translatorApi()?.create === 'function' || offlineTranslationSupported()
}

/**
 * Detect a BCP-47 primary language locally. Detection is deliberately bounded;
 * long message bodies do not need to be loaded into the detector in full.
 *
 * @param {string} text
 * @returns {Promise<{ language: string, confidence: number }>}
 */
export async function detectLanguage(text) {
  const sample = String(text || '').replace(/\s+/g, ' ').trim().slice(0, 2_000)
  if (sample.length < 20) return { language: '', confidence: 0 }
  if (typeof detectorApi()?.create === 'function') {
    try {
      const availability = typeof detectorApi().availability === 'function'
        ? await detectorApi().availability()
        : 'available'
      // Do not make a CADUCEUS installation depend on a browser vendor's model
      // download. Use the bundled detector whenever the native pack is absent.
      if (availability === 'available') {
        const detector = await detectorApi().create()
        try {
          const results = await detector.detect(sample)
          const best = Array.isArray(results) ? results[0] : null
          const language = normalizeTranslationLanguage(best?.detectedLanguage)
          if (language) return { language, confidence: Number(best?.confidence || 0) }
        } finally {
          detector.destroy?.()
        }
      }
    } catch {
      // The local trigram detector remains available if a native implementation
      // is present but fails to initialize.
    }
  }
  const candidates = francAll(sample, {
    only: Object.values(TRANSLATION_TO_FRANC_LANGUAGE),
    minLength: 20,
  })
  const [francLanguage, confidence] = candidates[0] || []
  const language = FRANC_TO_TRANSLATION_LANGUAGE[francLanguage] || ''
  return { language, confidence: language ? Number(confidence || 0) : 0 }
}

/**
 * Translate one bounded plain-text value using the browser-owned local model.
 * The model may be downloaded by the browser on first use, but message text is
 * never sent to CADUCEUS or a hosted translation API.
 *
 * @param {string} text
 * @param {{ sourceLanguage?: string, targetLanguage: string, onProgress?: (loaded: number) => void }} options
 */
export async function translateText(text, options) {
  const value = String(text || '')
  if (!value.trim()) return { text: value, sourceLanguage: '', targetLanguage: normalizeTranslationLanguage(options?.targetLanguage) }
  if (value.length > MAX_TRANSLATION_CHARACTERS) throw new Error(`Translation is limited to ${formatNumber(MAX_TRANSLATION_CHARACTERS)} characters.`)
  if (!onDeviceTranslationSupported()) throw new Error('On-device translation is not supported by this browser.')

  const targetLanguage = normalizeTranslationLanguage(options?.targetLanguage)
  if (!targetLanguage) throw new Error('Choose a supported target language.')
  let sourceLanguage = normalizeTranslationLanguage(options?.sourceLanguage)
  if (!sourceLanguage) sourceLanguage = (await detectLanguage(value)).language
  if (!sourceLanguage) throw new Error('CADUCEUS could not detect a supported source language.')
  if (sourceLanguage === targetLanguage) return { text: value, sourceLanguage, targetLanguage }
  const translator = await createTranslator(sourceLanguage, targetLanguage, options?.onProgress)
  try {
    return { text: await translator.translate(value), sourceLanguage, targetLanguage }
  } finally {
    translator.destroy?.()
  }
}

const SKIP_SELECTOR = 'script,style,code,pre,kbd,samp,blockquote,.notranslate,[translate="no"],[data-caduceus-signature],[data-template-variable]'
const PURE_URL = /^(?:(?:https?|ftp):\/\/|www\.)\S+$/iu
const US_ADDRESS = /(?:, [A-Z]{2},? [A-Z]{2,3},?|, [A-Za-z][A-Za-z .'-]+,? [A-Z]{2}) \d{5}(?:-\d{4})?/
const MONOSPACE_FONT = /(?:monospace|monaco|lucida console|courier)/i

/**
 * Preserve values that translation commonly damages: plaintext links,
 * compact US postal addresses, code-like monospace values, and tiny footer
 * copy. This mirrors the upstream reader's content-selection safeguards.
 *
 * @param {string} value
 * @param {{ fontFamily?: string, fontSize?: string }} [style]
 */
export function isTranslatableMessageText(value, style = {}) {
  const text = String(value || '').replace(/\s{2,}/g, ' ')
  if (!/[A-Za-z\p{L}]+/u.test(text) || text.trim().length <= 2) return false
  if (PURE_URL.test(text.trim())) return false
  if (text.length < 250 && US_ADDRESS.test(text)) return false
  if (MONOSPACE_FONT.test(String(style.fontFamily || ''))) return false
  const fontSize = Number.parseFloat(String(style.fontSize || ''))
  return !(Number.isFinite(fontSize) && fontSize > 0 && fontSize < 12)
}

/**
 * Build the detector sample from the same human-readable nodes translation
 * will process. Protected URLs, addresses, code, tiny footer copy, signatures,
 * and quoted replies must not vote for a different language.
 *
 * @param {string} html
 * @param {number} [limit]
 */
export function translationDetectionTextFromHtml(html, limit = 2_000) {
  if (typeof DOMParser !== 'function' || typeof NodeFilter === 'undefined') return ''
  const document = new DOMParser().parseFromString(String(html || ''), 'text/html')
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
  const parts = []
  let length = 0
  while (walker.nextNode() && length < limit) {
    const node = /** @type {Text} */ (walker.currentNode)
    const parent = node.parentElement
    const value = node.textContent || ''
    if (!parent || parent.closest(SKIP_SELECTOR) || !isTranslatableMessageText(value, textNodeStyle(parent))) continue
    const part = value.replace(/\s+/g, ' ').trim()
    if (!part) continue
    parts.push(part.slice(0, Math.max(0, limit - length)))
    length += part.length + 1
  }
  return parts.join(' ').slice(0, limit)
}

/** @param {Element} parent */
function textNodeStyle(parent) {
  const closestWithFont = parent.closest('[style*="font" i]')
  return closestWithFont instanceof HTMLElement
    ? { fontFamily: closestWithFont.style.fontFamily || closestWithFont.style.font || '', fontSize: closestWithFont.style.fontSize }
    : undefined
}

/**
 * Translate human-readable HTML text while preserving structure, links,
 * signatures, quoted replies, template variables, and code-like content.
 * Text nodes are processed in bounded chunks because browser translators run
 * requests sequentially and language packs have implementation quotas.
 *
 * @param {string} html
 * @param {{ sourceLanguage?: string, targetLanguage: string, onProgress?: (loaded: number) => void }} options
 */
export async function translateHtml(html, options) {
  if (typeof DOMParser !== 'function') throw new Error('HTML translation is unavailable in this browser.')
  const document = new DOMParser().parseFromString(String(html || ''), 'text/html')
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
  /** @type {Text[]} */
  const nodes = []
  let total = 0
  while (walker.nextNode()) {
    const node = /** @type {Text} */ (walker.currentNode)
    const parent = node.parentElement
    const value = node.textContent || ''
    if (!parent || parent.closest(SKIP_SELECTOR)) continue
    if (!isTranslatableMessageText(value, textNodeStyle(parent))) continue
    total += value.length
    if (total > MAX_TRANSLATION_CHARACTERS) throw new Error(`Translation is limited to ${formatNumber(MAX_TRANSLATION_CHARACTERS)} characters.`)
    nodes.push(node)
  }
  if (!nodes.length) return { html: document.body.innerHTML, sourceLanguage: '', targetLanguage: normalizeTranslationLanguage(options?.targetLanguage) }

  const sample = nodes.map(node => node.textContent || '').join(' ').slice(0, 2_000)
  let sourceLanguage = normalizeTranslationLanguage(options?.sourceLanguage)
  if (!sourceLanguage) sourceLanguage = (await detectLanguage(sample)).language
  if (!sourceLanguage) throw new Error('CADUCEUS could not detect a supported source language.')
  const targetLanguage = normalizeTranslationLanguage(options?.targetLanguage)
  if (!targetLanguage) throw new Error('Choose a supported target language.')
  if (sourceLanguage === targetLanguage) return { html: document.body.innerHTML, sourceLanguage, targetLanguage }

  const translator = await createTranslator(sourceLanguage, targetLanguage, options?.onProgress)
  try {
    for (const node of nodes) {
      const original = node.textContent || ''
      const leading = original.match(/^\s*/)?.[0] || ''
      const trailing = original.match(/\s*$/)?.[0] || ''
      const core = original.slice(leading.length, original.length - trailing.length)
      if (!core) continue
      node.textContent = `${leading}${await translator.translate(core)}${trailing}`
    }
  } finally {
    translator.destroy?.()
  }
  return { html: document.body.innerHTML, sourceLanguage, targetLanguage }
}

/** @param {string} sourceLanguage @param {string} targetLanguage @param {((loaded: number) => void) | undefined} onProgress */
async function createTranslator(sourceLanguage, targetLanguage, onProgress) {
  if (!onDeviceTranslationSupported()) throw new Error('On-device translation is not supported by this browser.')
  const native = translatorApi()
  if (typeof native?.create === 'function') {
    const configuration = { sourceLanguage, targetLanguage }
    try {
      const availability = typeof native.availability === 'function'
        ? await native.availability(configuration)
        : 'available'
      if (availability === 'available') {
        return native.create({
          ...configuration,
          monitor(monitor) {
            monitor.addEventListener('downloadprogress', event => onProgress?.(Number(event.loaded || 0)))
          },
        })
      }
    } catch {
      // Fall through to the CADUCEUS-hosted WASM model.
    }
  }
  return createCachedOfflineTranslator(sourceLanguage, targetLanguage, onProgress)
}

/** @param {string} sourceLanguage @param {string} targetLanguage @param {((loaded: number) => void) | undefined} onProgress */
async function createCachedOfflineTranslator(sourceLanguage, targetLanguage, onProgress) {
  const translator = await createOfflineTranslator(sourceLanguage, targetLanguage, onProgress)
  return {
    async translate(value) {
      const key = `${sourceLanguage}\0${targetLanguage}\0${value}`
      const cached = translatedSnippets.get(key)
      if (cached !== undefined) {
        translatedSnippets.delete(key)
        translatedSnippets.set(key, cached)
        return cached
      }
      const translated = await translator.translate(value)
      translatedSnippets.set(key, translated)
      while (translatedSnippets.size > TRANSLATED_SNIPPETS_LIMIT) {
        const oldest = translatedSnippets.keys().next().value
        if (oldest === undefined) break
        translatedSnippets.delete(oldest)
      }
      return translated
    },
    destroy() { translator.destroy?.() },
  }
}
