import { describe, expect, test } from 'bun:test'
import {
  loadCachedMessageTranslation,
  saveCachedMessageTranslation,
  setCachedMessageTranslationEnabled,
  TRANSLATION_CACHE_INDEX_KEY,
  TRANSLATION_CACHE_LIMIT,
} from '../../client/shared/scripts/translation-cache.js'

class MemoryStorage {
  values = new Map()
  get length() { return this.values.size }
  clear() { this.values.clear() }
  getItem(key) { return this.values.get(key) ?? null }
  key(index) { return [...this.values.keys()][index] ?? null }
  removeItem(key) { this.values.delete(key) }
  setItem(key, value) { this.values.set(key, String(value)) }
}

const record = messageId => ({
  messageId,
  sourceLanguage: 'es',
  targetLanguage: 'en',
  htmlBody: `<p>translated ${messageId}</p>`,
  plainText: `translated ${messageId}`,
  enabled: true,
})

describe('translated message cache', () => {
  test('restores only the requested target and persists the original/translated toggle', () => {
    const storage = new MemoryStorage()
    expect(saveCachedMessageTranslation(record('message-1'), storage)).toBe(true)
    expect(loadCachedMessageTranslation('message-1', 'fr', storage)).toBeNull()
    expect(loadCachedMessageTranslation('message-1', 'en', storage)).toMatchObject({ enabled: true, sourceLanguage: 'es' })
    expect(setCachedMessageTranslationEnabled('message-1', false, storage)).toBe(true)
    expect(loadCachedMessageTranslation('message-1', 'en', storage)?.enabled).toBe(false)
  })

  test('retains at most 150 recently used message translations', () => {
    const storage = new MemoryStorage()
    for (let index = 0; index < TRANSLATION_CACHE_LIMIT + 2; index += 1) {
      saveCachedMessageTranslation(record(`message-${index}`), storage)
    }
    const ids = JSON.parse(storage.getItem(TRANSLATION_CACHE_INDEX_KEY))
    expect(ids).toHaveLength(TRANSLATION_CACHE_LIMIT)
    expect(loadCachedMessageTranslation('message-0', 'en', storage)).toBeNull()
    expect(loadCachedMessageTranslation(`message-${TRANSLATION_CACHE_LIMIT + 1}`, 'en', storage)).not.toBeNull()
  })

  test('drops corrupt records without throwing', () => {
    const storage = new MemoryStorage()
    storage.setItem('caduceus-translated-message-v1:broken', '{')
    storage.setItem(TRANSLATION_CACHE_INDEX_KEY, JSON.stringify(['broken']))
    expect(loadCachedMessageTranslation('broken', 'en', storage)).toBeNull()
    expect(JSON.parse(storage.getItem(TRANSLATION_CACHE_INDEX_KEY))).toEqual([])
  })
})
