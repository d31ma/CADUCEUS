import {
  detectLanguage,
  translateHtml,
  translateText,
  translationDetectionTextFromHtml,
  TRANSLATION_LANGUAGES,
} from '../shared/scripts/on-device-translation.js'
import {
  resetTranslationPreferences,
  setTranslationPreference,
  translationPreferenceFor,
} from '../shared/scripts/translation-preferences.js'
import {
  loadCachedMessageTranslation,
  saveCachedMessageTranslation,
  setCachedMessageTranslationEnabled,
} from '../shared/scripts/translation-cache.js'
import { setTranslationAssetToken } from '../shared/scripts/chunked-asset-fetch.js'

/**
 * Escape hatch: the on-device translation engine of the message reader.
 * Language detection, the vendored WASM models, the DOM walk that translates
 * HTML text nodes, the localStorage cache and the per-language preferences
 * stay in JavaScript; MessageService keeps the reader's translation state.
 * The message to translate is read from the hidden [data-translation-source]
 * element, and translated text goes back in TranslationPart events of at
 * most 8000 characters, because one event holds at most 64 KiB.
 */
@Service
export default class TranslationEngineService {
  static _PART_CHARACTERS = 8000
  static _DWELL_MILLISECONDS = 500
  static _SAMPLE_CHARACTERS = 2000
  static _MINIMUM_SAMPLE = 20

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  languages = Object.entries(TRANSLATION_LANGUAGES).map(([id, label]) => ({ id, label }))

  /** @param {SessionChanged} changed */
  @OnEvent({"replay":true})
  session(changed) {
    setTranslationAssetToken(changed.token)
  }

  /**
   * @param {string} language
   * @param {string} fallback
   * @returns {string}
   */
  label(language, fallback) {
    return [fallback, TRANSLATION_LANGUAGES[language]][Number(Object.hasOwn(TRANSLATION_LANGUAGES, language))]
  }

  /** @param {Event} _event */
  always(_event) {
    this._choose(this._source().language, 'always', { translate: true })
  }

  /** @param {Event} _event */
  stop(_event) {
    this._choose(this._source().language, 'offer', {})
  }

  /** @param {Event} _event */
  never(_event) {
    this._choose(this._source().language, 'never', { original: true })
  }

  /** @param {Event} _event */
  resetPreferences(_event) {
    resetTranslationPreferences()
    this._preference(translationPreferenceFor(this._source().language), {})
  }

  /** @param {TranslationRestoreRequested} requested */
  @OnEvent
  restore(requested) {
    const cached = loadCachedMessageTranslation(requested.messageId, requested.target)
    ;[() => this._detectWhen(requested), () => this._restore(/** @type {CachedMessageTranslation} */ (cached))][Number(Boolean(cached))]()
  }

  /** @param {TranslationRequested} requested */
  @OnEvent
  async translate(requested) {
    await this._translate(requested).catch(error => this._failed(requested.messageId, error))
  }

  /** @param {TranslationVisibilityChanged} changed */
  @OnEvent
  visibility(changed) {
    setCachedMessageTranslationEnabled(changed.messageId, changed.enabled)
  }

  /** @param {string} source @param {string} preference @param {{translate?: boolean, original?: boolean}} next */
  _choose(source, preference, next) {
    if (!source) return
    setTranslationPreference(source, preference)
    this._preference(preference, next)
  }

  /** @param {string} preference @param {{translate?: boolean, original?: boolean}} next */
  _preference(preference, next) {
    /** @type {TranslationPreferenceChanged} */
    const changed = { preference, translate: Boolean(next.translate), original: Boolean(next.original) }
    this.applicationEventPublisher.publishEvent(changed, false)
  }

  /** @param {CachedMessageTranslation} cached */
  _restore(cached) {
    this._send(cached.messageId, cached.htmlBody, cached.plainText, cached.sourceLanguage)
    this._preference(translationPreferenceFor(cached.sourceLanguage), { original: !cached.enabled })
  }

  /** @param {TranslationRestoreRequested} requested */
  _detectWhen(requested) {
    if (requested.detect) setTimeout(() => { void this._detect(requested.messageId) }, TranslationEngineService._DWELL_MILLISECONDS)
  }

  /** @returns {{ messageId: string, html: string, text: string, language: string }} */
  _source() {
    const element = /** @type {HTMLElement | null} */ (document.querySelector('[data-translation-source]'))
    return { messageId: '', html: '', text: '', language: '', ...element?.dataset }
  }

  /** @param {string} messageId */
  async _detect(messageId) {
    const source = this._source()
    const sample = this._first([translationDetectionTextFromHtml(source.html), source.text]).replace(/\s+/g, ' ').trim().slice(0, TranslationEngineService._SAMPLE_CHARACTERS)
    await this._detectSample(messageId, [sample.length >= TranslationEngineService._MINIMUM_SAMPLE, source.messageId === messageId].every(Boolean), sample)
  }

  /** @param {string} messageId @param {boolean} usable @param {string} sample */
  async _detectSample(messageId, usable, sample) {
    if (!usable) return
    const result = await detectLanguage(sample)
    const language = this._first([result.language])
    /** @type {TranslationDetected} */
    const detected = { messageId, language, confidence: [Number(result.confidence)].filter(Number.isFinite).concat(0)[0], preference: translationPreferenceFor(language) }
    this.applicationEventPublisher.publishEvent(detected, false)
  }

  /** @param {unknown[]} values @returns {string} */
  _first(values) {
    return values.filter(value => typeof value === 'string').filter(Boolean).concat('')[0]
  }

  /** @param {TranslationRequested} requested */
  async _translate(requested) {
    const source = this._source()
    const onProgress = (/** @type {number} */ progress) => this._progress(progress)
    const options = { sourceLanguage: requested.source, targetLanguage: requested.target, onProgress }
    const html = await this._translatedHtml(source.html, options)
    const text = await this._translatedText(source.text, { ...options, sourceLanguage: this._first([html.sourceLanguage, options.sourceLanguage]) })
    const language = this._first([html.sourceLanguage, text.sourceLanguage])
    this._requireText([html.html, text.text])
    this._send(requested.messageId, html.html, text.text, language)
    saveCachedMessageTranslation({ messageId: requested.messageId, sourceLanguage: language, targetLanguage: requested.target, htmlBody: html.html, plainText: text.text, enabled: true })
  }

  /** @param {string} html @param {{ sourceLanguage: string, targetLanguage: string, onProgress: (progress: number) => void }} options */
  async _translatedHtml(html, options) {
    return html ? translateHtml(html, options) : { html: '', sourceLanguage: '' }
  }

  /** @param {string} text @param {{ sourceLanguage: string, targetLanguage: string, onProgress: (progress: number) => void }} options */
  async _translatedText(text, options) {
    return text ? translateText(text, options) : { text: '', sourceLanguage: '' }
  }

  /** @param {string[]} translated */
  _requireText(translated) {
    if (!translated.some(Boolean)) throw new Error('This message has no translatable text.')
  }

  /** @param {string} messageId @param {unknown} error */
  _failed(messageId, error) {
    /** @type {TranslationFailed} */
    const failed = { messageId, message: error instanceof Error ? error.message : '' }
    this.applicationEventPublisher.publishEvent(failed, false)
  }

  /** @param {number} progress */
  _progress(progress) {
    /** @type {TranslationProgressed} */
    const progressed = { progress }
    this.applicationEventPublisher.publishEvent(progressed, false)
  }

  /** @param {string} messageId @param {string} html @param {string} text @param {string} source */
  _send(messageId, html, text, source) {
    this._sendParts(messageId, 'html', String(html))
    this._sendParts(messageId, 'text', String(text))
    /** @type {TranslationPart} */
    const done = { messageId, kind: 'done', index: 0, count: 1, text: source }
    this.applicationEventPublisher.publishEvent(done, false)
  }

  /** @param {string} messageId @param {string} kind @param {string} value */
  _sendParts(messageId, kind, value) {
    const size = TranslationEngineService._PART_CHARACTERS
    const points = Array.from(value)
    const count = Math.ceil(points.length / size)
    for (let index = 0; index < count; index++) {
      /** @type {TranslationPart} */
      const part = { messageId, kind, index, count, text: points.slice(index * size, (index + 1) * size).join('') }
      this.applicationEventPublisher.publishEvent(part, false)
    }
  }
}
