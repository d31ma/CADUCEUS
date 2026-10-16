// @ts-check

import { normalizeTranslationLanguage } from './on-device-translation.js'

export const TRANSLATION_CACHE_INDEX_KEY = 'caduceus-translated-message-index-v1'
export const TRANSLATION_CACHE_PREFIX = 'caduceus-translated-message-v1:'
export const TRANSLATION_CACHE_LIMIT = 150

/**
 * Load and LRU-touch one local translation. A different target language is a
 * cache miss so changing the reader target can never show stale content.
 *
 * @param {string} messageId
 * @param {string} targetLanguage
 * @param {Storage | undefined} [storage]
 * @returns {CachedMessageTranslation | null}
 */
export function loadCachedMessageTranslation(messageId, targetLanguage, storage = globalThis.localStorage) {
  const id = normalizeMessageId(messageId)
  const target = normalizeTranslationLanguage(targetLanguage)
  if (!id || !target || !storage) return null
  try {
    const parsed = normalizeRecord(JSON.parse(storage.getItem(cacheKey(id)) || 'null'))
    if (!parsed || parsed.messageId !== id || parsed.targetLanguage !== target) return null
    touchIndex(id, storage)
    return parsed
  } catch {
    removeCachedMessageTranslation(id, storage)
    return null
  }
}

/**
 * Persist one translated message and retain only the 150 most recently used
 * entries, matching Mailspring's bounded local cache without storing mail on a
 * hosted service.
 *
 * @param {CachedMessageTranslation} value
 * @param {Storage | undefined} [storage]
 */
export function saveCachedMessageTranslation(value, storage = globalThis.localStorage) {
  if (!storage) return false
  const record = normalizeRecord(value)
  if (!record || (!record.htmlBody && !record.plainText)) return false
  const key = cacheKey(record.messageId)
  const payload = JSON.stringify(record)
  let index = readIndex(storage).filter(id => id !== record.messageId)

  // A mailbox can contain unusually large HTML bodies. If browser storage is
  // full, evict the least-recently-used translations until this bounded entry
  // fits; failure remains local and does not break the reader.
  while (true) {
    try {
      storage.setItem(key, payload)
      break
    } catch {
      const oldest = index.shift()
      if (!oldest) return false
      storage.removeItem(cacheKey(oldest))
    }
  }
  index.push(record.messageId)
  while (index.length > TRANSLATION_CACHE_LIMIT) {
    const oldest = index.shift()
    if (oldest) storage.removeItem(cacheKey(oldest))
  }
  writeIndex(index, storage)
  return true
}

/** @param {string} messageId @param {boolean} enabled @param {Storage | undefined} [storage] */
export function setCachedMessageTranslationEnabled(messageId, enabled, storage = globalThis.localStorage) {
  const id = normalizeMessageId(messageId)
  if (!id || !storage) return false
  try {
    const record = normalizeRecord(JSON.parse(storage.getItem(cacheKey(id)) || 'null'))
    if (!record) return false
    return saveCachedMessageTranslation({ ...record, enabled: Boolean(enabled) }, storage)
  } catch {
    removeCachedMessageTranslation(id, storage)
    return false
  }
}

/** @param {string} messageId @param {Storage | undefined} [storage] */
export function removeCachedMessageTranslation(messageId, storage = globalThis.localStorage) {
  const id = normalizeMessageId(messageId)
  if (!id || !storage) return
  try {
    storage.removeItem(cacheKey(id))
    writeIndex(readIndex(storage).filter(value => value !== id), storage)
  } catch {}
}

/** @param {unknown} value @returns {CachedMessageTranslation | null} */
function normalizeRecord(value) {
  const messageId = normalizeMessageId(/** @type {any} */ (value)?.messageId)
  const sourceLanguage = normalizeTranslationLanguage(/** @type {any} */ (value)?.sourceLanguage)
  const targetLanguage = normalizeTranslationLanguage(/** @type {any} */ (value)?.targetLanguage)
  if (!messageId || !sourceLanguage || !targetLanguage || sourceLanguage === targetLanguage) return null
  return {
    messageId,
    sourceLanguage,
    targetLanguage,
    htmlBody: typeof /** @type {any} */ (value).htmlBody === 'string' ? /** @type {any} */ (value).htmlBody : '',
    plainText: typeof /** @type {any} */ (value).plainText === 'string' ? /** @type {any} */ (value).plainText : '',
    enabled: /** @type {any} */ (value).enabled !== false,
  }
}

/** @param {string} id */
function cacheKey(id) { return `${TRANSLATION_CACHE_PREFIX}${encodeURIComponent(id)}` }

/** @param {unknown} value */
function normalizeMessageId(value) { return String(value || '').trim().slice(0, 512) }

/** @param {Storage} storage */
function readIndex(storage) {
  try {
    const parsed = JSON.parse(storage.getItem(TRANSLATION_CACHE_INDEX_KEY) || '[]')
    return Array.isArray(parsed) ? [...new Set(parsed.map(normalizeMessageId).filter(Boolean))] : []
  } catch {
    return []
  }
}

/** @param {string} id @param {Storage} storage */
function touchIndex(id, storage) {
  const index = readIndex(storage).filter(value => value !== id)
  index.push(id)
  writeIndex(index, storage)
}

/** @param {string[]} index @param {Storage} storage */
function writeIndex(index, storage) {
  try { storage.setItem(TRANSLATION_CACHE_INDEX_KEY, JSON.stringify(index.slice(-TRANSLATION_CACHE_LIMIT))) } catch {}
}
