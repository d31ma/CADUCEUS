/**
 * Per-language translation behavior that stays on this device. The choices
 * live under the fixed localStorage key that the message reader also reads,
 * so this stays a JavaScript service: a WebAssembly service's saved fields
 * are kept under Tachyon's own per-page keys.
 */
@Service
export default class TranslationPreferencesService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;
  /** @type {string[]} */
  _automatic = []
  /** @type {string[]} */
  _disabled = []

  static _KEY = 'caduceus-translation-preferences-v1'
  static _EVENT = 'caduceus:translation-preferences'

  @OnStart
  start() { this._load() }

  get automaticRows() { return TranslationPreferencesService._rows(this._automatic) }

  get disabledRows() { return TranslationPreferencesService._rows(this._disabled) }

  /** @param {Event} _event @param {string} language */
  restore(_event, language) {
    const current = TranslationPreferencesService._stored()
    TranslationPreferencesService._write({
      automatic: current.automatic.filter(value => value !== language),
      disabled: current.disabled.filter(value => value !== language),
    })
    this._load()
  }

  resetAll() {
    TranslationPreferencesService._clear()
    this._load()
    /** @type {ToastRequested} */
    const toast = { message: 'Translation preferences reset.' }
    this.applicationEventPublisher.publishEvent(toast, false)
  }

  _load() {
    const stored = TranslationPreferencesService._stored()
    this._disabled = stored.disabled
    this._automatic = stored.automatic.filter(language => !stored.disabled.includes(language))
  }

  /** @returns {{ automatic: string[], disabled: string[] }} */
  static _stored() {
    const parsed = Object(TranslationPreferencesService._parse())
    return { automatic: TranslationPreferencesService._list(parsed.automatic), disabled: TranslationPreferencesService._list(parsed.disabled) }
  }

  /** @returns {Record<string, unknown>} */
  static _parse() {
    try { return JSON.parse(String(localStorage.getItem(TranslationPreferencesService._KEY))) } catch { return {} }
  }

  /** @param {unknown} value @returns {string[]} */
  static _list(value) {
    return Array.isArray(value) ? [...new Set(value.map(item => String(item).trim().toLowerCase().split('-')[0]))] : []
  }

  /** @param {{ automatic: string[], disabled: string[] }} next */
  static _write(next) {
    try { localStorage.setItem(TranslationPreferencesService._KEY, JSON.stringify(next)) } catch {}
    window.dispatchEvent(new CustomEvent(TranslationPreferencesService._EVENT, { detail: next }))
  }

  static _clear() {
    try { localStorage.removeItem(TranslationPreferencesService._KEY) } catch {}
    window.dispatchEvent(new CustomEvent(TranslationPreferencesService._EVENT, { detail: { automatic: [], disabled: [] } }))
  }

  /** @param {string[]} languages */
  static _rows(languages) {
    return languages.map(id => ({ id, label: TranslationPreferencesService._label(id) }))
  }

  /** @param {string} id */
  static _label(id) {
    try { return String(new Intl.DisplayNames(['en'], { type: 'language' }).of(id)) } catch { return id }
  }
}
