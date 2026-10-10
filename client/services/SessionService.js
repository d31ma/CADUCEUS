/**
 * Minimal stand-in for the SessionService that the shell area owns. It reads
 * the signed-in session that the login flow keeps in sessionStorage and
 * publishes it once, retained, so services in any language can hear it with
 * `replay` and take their bearer token from it.
 */
@Service
export default class SessionService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;
  _token = ''
  _email = ''
  _role = ''
  /** @type {string[]} */
  _domains = []

  @OnStart
  start() {
    this._token = SessionService._text(SessionService._read('caduceus_token'))
    this._email = SessionService._text(SessionService._read('caduceus_email'))
    this._role = SessionService._text(SessionService._read('caduceus_role'))
    this._domains = SessionService._list(SessionService._parse(SessionService._read('caduceus_domains')))
    /** @type {AccountSession} */
    const session = { token: this._token, email: this._email, role: this._role, domains: this._domains }
    this.applicationEventPublisher.publishEvent(session, true)
  }

  get signedIn() { return this._token !== '' }

  get signedOut() { return this._token === '' }

  /** @param {string} key @returns {string | null} */
  static _read(key) {
    try { return sessionStorage.getItem(key) } catch { return null }
  }

  /** @param {string | null} value */
  static _text(value) { return value === null ? '' : value }

  /** @param {string | null} value @returns {unknown} */
  static _parse(value) {
    try { return JSON.parse(String(value)) } catch { return null }
  }

  /** @param {unknown} value @returns {string[]} */
  static _list(value) { return Array.isArray(value) ? value.map(String) : [] }
}
