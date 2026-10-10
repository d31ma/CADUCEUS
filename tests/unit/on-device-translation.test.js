import { afterEach, describe, expect, test } from 'bun:test'
import { OnDeviceTranslationUtils } from '../../client/shared/utils/OnDeviceTranslationUtils.js'
import { OfflineTranslationEngineUtils } from '../../client/shared/utils/OfflineTranslationEngineUtils.js'
import { TranslationLanguagesUtils } from '../../client/shared/utils/TranslationLanguagesUtils.js'
import TranslationService from '../../client/services/TranslationService.js'

const detectLanguage = (...args) => OnDeviceTranslationUtils.detectLanguage(...args)
const isTranslatableMessageText = (...args) => OnDeviceTranslationUtils.isTranslatableMessageText(...args)
const MAX_TRANSLATION_CHARACTERS = OnDeviceTranslationUtils.maxTranslationCharacters()
const normalizeTranslationLanguage = (...args) => OnDeviceTranslationUtils.normalizeTranslationLanguage(...args)
const onDeviceTranslationSupported = (...args) => OnDeviceTranslationUtils.onDeviceTranslationSupported(...args)
const preferredTranslationLanguage = (...args) => OnDeviceTranslationUtils.preferredTranslationLanguage(...args)
const TRANSLATION_LANGUAGES = OnDeviceTranslationUtils.translationLanguages()
// translateText moved to TranslationService; its shell is only used to fetch self-hosted model assets.
const translation = Object.assign(new TranslationService(), {
  shell: { apiFetch: async path => { throw new Error(`unexpected translation asset fetch: ${path}`) } },
})
const translateText = (...args) => translation.translateText(...args)
const OFFLINE_TRANSLATION_CHUNK_CHARACTERS = OfflineTranslationEngineUtils.offlineTranslationChunkCharacters()
const buildOfflineTranslationPlan = (...args) => OfflineTranslationEngineUtils.buildOfflineTranslationPlan(...args)
const prependOpusTargetToken = (...args) => OfflineTranslationEngineUtils.prependOpusTargetToken(...args)
const prependSmall100TargetToken = (...args) => OfflineTranslationEngineUtils.prependSmall100TargetToken(...args)
const splitOfflineTranslationText = (...args) => OfflineTranslationEngineUtils.splitOfflineTranslationText(...args)
const FRANC_TO_TRANSLATION_LANGUAGE = TranslationLanguagesUtils.francToTranslationLanguage()
const OPUS_EXTENSION_LANGUAGE_CODES = TranslationLanguagesUtils.opusExtensionLanguageCodes()

const originalTranslator = globalThis.Translator
const originalDetector = globalThis.LanguageDetector
const originalOfflineFactory = globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY

afterEach(() => {
  if (originalTranslator === undefined) delete globalThis.Translator
  else globalThis.Translator = originalTranslator
  if (originalDetector === undefined) delete globalThis.LanguageDetector
  else globalThis.LanguageDetector = originalDetector
  if (originalOfflineFactory === undefined) delete globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY
  else globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY = originalOfflineFactory
})

describe('on-device translation adapter', () => {
  test('normalizes supported BCP-47 language tags and defaults safely', () => {
    expect(normalizeTranslationLanguage('FR-ca')).toBe('fr')
    expect(normalizeTranslationLanguage('fi-FI')).toBe('fi')
    expect(normalizeTranslationLanguage('../fr')).toBe('')
    expect(preferredTranslationLanguage('xx-ZZ')).toBe('en')
    expect(Object.keys(TRANSLATION_LANGUAGES)).toHaveLength(93)
    expect(OPUS_EXTENSION_LANGUAGE_CODES).toEqual([
      'mt', 'mi', 'mhr', 'eu', 'pap', 'mrj', 'tg', 'tt', 'te', 'udm', 'ky', 'la', 'eo',
    ])
    expect(new Set(Object.values(FRANC_TO_TRANSLATION_LANGUAGE)).isSubsetOf(
      new Set(Object.keys(TRANSLATION_LANGUAGES)),
    )).toBe(true)
  })

  test('detects a Mailspring reader language outside the composer subset', async () => {
    delete globalThis.LanguageDetector
    expect(await detectLanguage('Tämä viesti sisältää riittävästi suomenkielistä tekstiä luotettavaa tunnistusta varten.'))
      .toMatchObject({ language: 'fi' })
  })

  test('detects and translates locally while releasing browser model resources', async () => {
    let detectorDestroyed = false
    let translatorDestroyed = false
    globalThis.LanguageDetector = {
      availability: async () => 'available',
      create: async () => ({
        detect: async () => [{ detectedLanguage: 'es-MX', confidence: 0.98 }],
        destroy: () => { detectorDestroyed = true },
      }),
    }
    globalThis.Translator = {
      availability: async options => options.sourceLanguage === 'es' && options.targetLanguage === 'en' ? 'available' : 'unavailable',
      create: async options => ({
        translate: async text => `${options.sourceLanguage}:${options.targetLanguage}:${text}`,
        destroy: () => { translatorDestroyed = true },
      }),
    }

    expect(onDeviceTranslationSupported()).toBe(true)
    expect(await detectLanguage('Hola, este mensaje tiene suficiente texto para detectarse.')).toEqual({ language: 'es', confidence: 0.98 })
    const result = await translateText('Hola, este mensaje tiene suficiente texto para detectarse.', { targetLanguage: 'en' })
    expect(result).toEqual({
      text: 'es:en:Hola, este mensaje tiene suficiente texto para detectarse.',
      sourceLanguage: 'es',
      targetLanguage: 'en',
    })
    expect(detectorDestroyed).toBe(true)
    expect(translatorDestroyed).toBe(true)
  })

  test('falls back to the bundled detector and multilingual engine', async () => {
    delete globalThis.Translator
    delete globalThis.LanguageDetector
    const progress = []
    globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY = async options => {
      options.onProgress?.(0.5)
      return {
        translate: async text => `[offline ${options.sourceLanguage}→${options.targetLanguage}] ${text}`,
      }
    }

    expect(await detectLanguage('Hola, este mensaje contiene información importante para todos.'))
      .toMatchObject({ language: 'es' })
    const result = await translateText('Hola, este mensaje contiene información importante para todos.', {
      targetLanguage: 'en',
      onProgress: value => progress.push(value),
    })
    expect(result).toEqual({
      text: '[offline es→en] Hola, este mensaje contiene información importante para todos.',
      sourceLanguage: 'es',
      targetLanguage: 'en',
    })
    expect(progress).toEqual([0.5])
  })

  test('uses the bundled engine when a native pair would require a download', async () => {
    let nativeCreateCalls = 0
    globalThis.Translator = {
      availability: async () => 'downloadable',
      create: async () => { nativeCreateCalls += 1; return {} },
    }
    globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY = async options => ({
      translate: async text => `${options.sourceLanguage}:${options.targetLanguage}:${text}`,
    })
    expect(await translateText('Bonjour tout le monde', { sourceLanguage: 'fr', targetLanguage: 'en' }))
      .toEqual({ text: 'fr:en:Bonjour tout le monde', sourceLanguage: 'fr', targetLanguage: 'en' })
    expect(nativeCreateCalls).toBe(0)
  })

  test('fails closed for local engine failures and oversized text', async () => {
    delete globalThis.Translator
    globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY = async () => {
      throw new Error('local model unavailable')
    }
    await expect(translateText('Bonjour tout le monde', { sourceLanguage: 'fr', targetLanguage: 'en' }))
      .rejects.toThrow('local model unavailable')
    await expect(translateText('x'.repeat(MAX_TRANSLATION_CHARACTERS + 1), { sourceLanguage: 'en', targetLanguage: 'fr' }))
      .rejects.toThrow('limited')
  })

  test('chunks long model inputs without losing paragraph separators', () => {
    const sentence = 'This sentence is intentionally long enough to exercise bounded model input. '
    const text = sentence.repeat(30) + '\n\n' + sentence.repeat(2)
    const pieces = splitOfflineTranslationText(text)
    expect(pieces.map(piece => piece.text).join('')).toBe(text)
    expect(pieces.filter(piece => piece.translatable).every(piece => piece.text.length <= OFFLINE_TRANSLATION_CHUNK_CHARACTERS)).toBe(true)
    expect(pieces.some(piece => !piece.translatable && piece.text === '\n\n')).toBe(true)
  })

  test('selects a SMaLL-100 target by prepending its locked language token', () => {
    expect(prependSmall100TargetToken([42n, 2n], 'fr', { en: 128022, fr: 128038 }))
      .toEqual([128038n, 42n, 2n])
    expect(() => prependSmall100TargetToken([42n, 2n], 'xx', { en: 128022 }))
      .toThrow('Unsupported self-hosted translation target')
  })

  test('routes extension languages through the smallest required local models', () => {
    expect(buildOfflineTranslationPlan('es', 'fi')).toEqual([
      { model: 'small100', sourceLanguage: 'es', targetLanguage: 'fi' },
    ])
    expect(buildOfflineTranslationPlan('en', 'eo')).toEqual([
      { model: 'opus-en-mul', sourceLanguage: 'en', targetLanguage: 'eo' },
    ])
    expect(buildOfflineTranslationPlan('eu', 'en')).toEqual([
      { model: 'opus-mul-en', sourceLanguage: 'eu', targetLanguage: 'en' },
    ])
    expect(buildOfflineTranslationPlan('te', 'la')).toEqual([
      { model: 'opus-mul-en', sourceLanguage: 'te', targetLanguage: 'en' },
      { model: 'opus-en-mul', sourceLanguage: 'en', targetLanguage: 'la' },
    ])
    expect(buildOfflineTranslationPlan('fr', 'tt')).toEqual([
      { model: 'small100', sourceLanguage: 'fr', targetLanguage: 'en' },
      { model: 'opus-en-mul', sourceLanguage: 'en', targetLanguage: 'tt' },
    ])
    expect(buildOfflineTranslationPlan('mrj', 'en')).toEqual([
      { model: 'opus-mul-en', sourceLanguage: 'mrj', targetLanguage: 'en' },
    ])
  })

  test('prepends the locked OPUS destination token without tokenizer prefix loss', () => {
    const vocabulary = new Map([['>>epo<<', 894], ['>>mhr<<', 925]])
    expect(prependOpusTargetToken([42n, 0n], 'eo', vocabulary)).toEqual([894n, 42n, 0n])
    expect(prependOpusTargetToken([42n, 0n], 'mrj', vocabulary)).toEqual([925n, 42n, 0n])
    expect(() => prependOpusTargetToken([42n], 'te', vocabulary)).toThrow('Unsupported OPUS translation target')
  })

  test('protects links, addresses, monospace values, and tiny footer text', () => {
    expect(isTranslatableMessageText('https://example.test/reset?token=secret')).toBe(false)
    expect(isTranslatableMessageText('Office, San Francisco, CA 94114')).toBe(false)
    expect(isTranslatableMessageText('API_TOKEN_VALUE', { fontFamily: 'Monaco, monospace' })).toBe(false)
    expect(isTranslatableMessageText('Manage notification preferences', { fontSize: '10px' })).toBe(false)
    expect(isTranslatableMessageText('Hola, este contenido sí debe traducirse.')).toBe(true)
  })
})
