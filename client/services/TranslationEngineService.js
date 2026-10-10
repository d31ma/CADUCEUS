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

/**
 * The on-device translation engine for the Go message reader. Language
 * detection (franc), the vendored WASM translation models, the DOM walk
 * that translates _HTML text nodes, the bounded localStorage cache and the
 * per-language preferences stay in JavaScript; MessageService keeps the
 * reader's translation state. Translated text goes back in parts, because
 * one call into a WebAssembly service holds at most 64 KiB.
 */
@Service
export default class TranslationEngineService {
  static _PART_CHARACTERS = 8000
  static _DWELL_MILLISECONDS = 500
  static _SAMPLE_CHARACTERS = 2000
  static _MINIMUM_SAMPLE = 20
  static _ALWAYS = 'always'
  static _HTML = 'html'
  static _TEXT = 'text'

  /** @type {TacTaskScheduler} */
  @Needs
  taskScheduler
  /** @type {MessageService} */
  message
  languages = Object.entries(TRANSLATION_LANGUAGES).map(([id, label]) => ({ id, label }))

  /**
   * @param {string} language
   * @param {string} fallback
   * @returns {string}
   */
  label(language, fallback) {
    return this._label(language, fallback)
  }

  /**
   * @param {Event} _event
   * @param {string} target
   */
  async changeTarget(_event, target) {
    await this._changeTarget(target)
  }

  /** @param {Event} _event */
  async always(_event) {
    await this._setPreference(TranslationEngineService._ALWAYS)
    await this.message.translateMessage()
  }

  /** @param {Event} _event */
  async stop(_event) {
    await this._setPreference('offer')
  }

  /** @param {Event} _event */
  async never(_event) {
    await this._setPreference('never')
    await this.message.showOriginalMessage()
  }

  /** @param {Event} _event */
  async resetPreferences(_event) {
    resetTranslationPreferences()
    await this.message.translationPreferenceChanged(translationPreferenceFor(this.message.translationSource))
  }

  /** @param {TranslationRestoreRequested} requested */
  @OnEvent
  async restore(requested) {
    const cached = loadCachedMessageTranslation(requested.messageId, requested.target)
    await this._restoreOrDetect(cached, requested)
    await this.taskScheduler.render()
  }

  /** @param {TranslationRequested} requested */
  @OnEvent
  async translate(requested) {
    await this._translate(requested)
    await this.taskScheduler.render()
  }

  /** @param {TranslationVisibilityChanged} changed */
  @OnEvent
  visibility(changed) {
    setCachedMessageTranslationEnabled(changed.messageId, changed.enabled)
  }

  /** @param {string} language @param {string} fallback @returns {string} */
  _label(language, fallback) {
    return Object.hasOwn(TRANSLATION_LANGUAGES, language) ? TRANSLATION_LANGUAGES[language] : fallback
  }

  /** @param {string} target */
  async _changeTarget(target) {
    if (!Object.hasOwn(TRANSLATION_LANGUAGES, target)) return
    await this.message.changeTranslationTarget(target)
  }

  /** @param {string} preference */
  async _setPreference(preference) {
    const source = this.message.translationSource
    if (!source) return
    setTranslationPreference(source, preference)
    await this.message.translationPreferenceChanged(preference)
  }

  /** @param {CachedMessageTranslation | null} cached @param {TranslationRestoreRequested} requested */
  async _restoreOrDetect(cached, requested) {
    if (cached) await this._restore(cached)
    else if (requested.detect) await this._detect(requested.messageId)
  }

  /** @param {CachedMessageTranslation} cached */
  async _restore(cached) {
    await this._send(cached.messageId, cached.htmlBody, cached.plainText, cached.sourceLanguage)
    await this.message.translationPreferenceChanged(translationPreferenceFor(cached.sourceLanguage))
    await this._hideUnless(cached.enabled)
  }

  /** @param {boolean} enabled */
  async _hideUnless(enabled) {
    if (!enabled) await this.message.showOriginalMessage()
  }

  /** @param {string} messageId */
  async _detect(messageId) {
    await new Promise(resolve => setTimeout(resolve, TranslationEngineService._DWELL_MILLISECONDS))
    const sample = this._sample(this.message.email)
    await this._detectSample(messageId, sample)
  }

  /** @param {Email} email @returns {string} */
  _sample(email) {
    const html = this._htmlSample(email?.htmlBody)
    const text = [html, this.message.plainTextVisible].filter(Boolean).concat('')[0]
    return String(text).replace(/\s+/g, ' ').trim().slice(0, TranslationEngineService._SAMPLE_CHARACTERS)
  }

  /** @param {string | undefined} html @returns {string} */
  _htmlSample(html) {
    return html ? translationDetectionTextFromHtml(html) : ''
  }

  /** @param {string} messageId @param {string} sample */
  async _detectSample(messageId, sample) {
    if (sample.length < TranslationEngineService._MINIMUM_SAMPLE || this.message.email?.id !== messageId) return
    const result = await detectLanguage(sample)
    await this.message.languageDetected(messageId, result.language, result.confidence, translationPreferenceFor(result.language))
  }

  /** @param {TranslationRequested} requested */
  async _translate(requested) {
    await this._translateMessage(requested).catch(error => this._failed(requested.messageId, error))
  }

  /** @param {string} messageId @param {unknown} error */
  async _failed(messageId, error) {
    await this.message.translationFailed(messageId, error instanceof Error ? error.message : '')
  }

  /** @param {TranslationRequested} requested */
  async _translateMessage(requested) {
    const onProgress = (/** @type {number} */ loaded) => { void this._progress(loaded) }
    const options = { sourceLanguage: requested.source, targetLanguage: requested.target, onProgress }
    const html = await this._translatedHtml(this._htmlBody(), options)
    const text = await this._translatedText(this._plainText(), { ...options, sourceLanguage: this._firstSource(html.sourceLanguage, options.sourceLanguage) })
    const source = this._firstSource(html.sourceLanguage, text.sourceLanguage)
    this._requireText(html.html, text.text)
    await this._send(requested.messageId, html.html, text.text, source)
    saveCachedMessageTranslation({ messageId: requested.messageId, sourceLanguage: source, targetLanguage: requested.target, htmlBody: html.html, plainText: text.text, enabled: true })
  }

  /** @returns {string} */
  _htmlBody() {
    return String(this.message.email?.htmlBody ?? '')
  }

  /** @returns {string} */
  _plainText() {
    return String(this.message.plainTextVisible ?? '')
  }

  /** @param {string} first @param {string} fallback @returns {string} */
  _firstSource(first, fallback) {
    return first || fallback
  }

  /** @param {string} html @param {{ sourceLanguage: string, targetLanguage: string, onProgress: (loaded: number) => void }} options */
  async _translatedHtml(html, options) {
    return html ? translateHtml(html, options) : { html: '', sourceLanguage: '' }
  }

  /** @param {string} text @param {{ sourceLanguage: string, targetLanguage: string, onProgress: (loaded: number) => void }} options */
  async _translatedText(text, options) {
    return text ? translateText(text, options) : { text: '', sourceLanguage: '' }
  }

  /** @param {string} html @param {string} text */
  _requireText(html, text) {
    if (!html && !text) throw new Error('This message has no translatable text.')
  }

  /** @param {number} loaded */
  async _progress(loaded) {
    await this.message.translationProgressed(loaded)
    await this.taskScheduler.render()
  }

  /** @param {string} messageId @param {string} html @param {string} text @param {string} source */
  async _send(messageId, html, text, source) {
    await this._sendParts(messageId, TranslationEngineService._HTML, String(html))
    await this._sendParts(messageId, TranslationEngineService._TEXT, String(text))
    await this.message.receiveTranslation({ messageId, kind: 'done', index: 0, count: 1, text: source })
  }

  /** @param {string} messageId @param {string} kind @param {string} value */
  async _sendParts(messageId, kind, value) {
    const size = TranslationEngineService._PART_CHARACTERS
    const points = Array.from(value)
    const parts = Array.from({ length: Math.ceil(points.length / size) }, (_, index) => points.slice(index * size, (index + 1) * size).join(''))
    for (const [index, text] of parts.entries()) await this.message.receiveTranslation({ messageId, kind, index, count: parts.length, text })
  }
}
