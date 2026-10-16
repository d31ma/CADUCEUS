/**
 * Escape hatch: the bridge between Go services and the tab's web storage.
 *
 * Go services cannot read sessionStorage or localStorage, and their own
 * SessionState and LocalState fields are kept per route path. The session and
 * the per-device preferences must outlive a navigation and keep their legacy
 * `caduceus*` keys (the e2e suites and unported pages read them), so this
 * service hands those keys to Go in one retained StoredValues event and writes
 * back every StorageWritten event.
 */
@Service
export default class BrowserStorageService {
  static _LOCAL = 'local'
  static _LARGEST_VALUE = 8192

  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher

  ready = false

  @OnStart
  start() {
    /** @type {StoredValues} */
    const stored = { session: this._read(sessionStorage), local: this._read(localStorage) }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(stored, true)
    this.ready = true
  }

  /** @param {StorageWritten} written */
  @OnEvent
  write(written) {
    const store = this._store(written.area)
    this._remove(store, written.removed)
    this._save(store, written.values)
  }

  /** @param {string} area */
  _store(area) {
    return area === BrowserStorageService._LOCAL ? localStorage : sessionStorage
  }

  /** @param {Storage} store */
  _read(store) {
    const keys = Object.keys(store).filter(key => key.startsWith('caduceus'))
    const entries = keys.map(key => [key, String(store.getItem(key))])
    return Object.fromEntries(entries.filter(entry => entry[1].length <= BrowserStorageService._LARGEST_VALUE))
  }

  /** @param {Storage} store @param {string[]} keys */
  _remove(store, keys) {
    for (const key of keys) store.removeItem(key)
  }

  /** @param {Storage} store @param {Record<string, string>} values */
  _save(store, values) {
    for (const [key, value] of Object.entries(values)) store.setItem(key, value)
  }
}
