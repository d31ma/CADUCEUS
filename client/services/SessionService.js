/**
 * STAND-IN for the shell area's SessionService (delete at merge: a JavaScript
 * service file wins over a Go one with the same name).
 *
 * The mailbox-list services are WebAssembly and cannot read sessionStorage or
 * inject another service, so they hear the sign-in state as one retained
 * `SessionChanged` event and the list preferences as `MailPreferencesChanged`.
 * This stand-in reads the keys the shell (and the e2e `loginAs` helper)
 * already writes and republishes when the shell announces new mailbox names
 * or connected accounts.
 */
@Service
export default class SessionService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  /** @type {AbortSignal} */
  @Needs
  lifecycle
  /** @type {boolean} */
  signedIn = false
  /** @type {ExternalAccount[]} */
  _externalAccounts = []
  /** @type {Record<string, ExternalFolder[]>} */
  _externalFolders = {}

  @OnStart
  start() {
    addEventListener('caduceus:mailbox-names', () => this._publish(), { signal: this.lifecycle })
    addEventListener('caduceus:external-mailboxes', (event) => this._connected(event), { signal: this.lifecycle })
    this._publish()
  }

  /** @param {Event} event */
  _connected(event) {
    const detail = this._record(/** @type {CustomEvent} */ (event).detail)
    this._externalAccounts = /** @type {ExternalAccount[]} */ (this._list(detail.accounts))
    this._externalFolders = /** @type {Record<string, ExternalFolder[]>} */ (this._record(detail.folders))
    this._publish()
  }

  _publish() {
    const preferences = this._record(this._parsed(this._stored('caduceus_mail_preferences')))
    /** @type {SessionChanged} */
    const session = {
      token: this._stored('caduceus_token'),
      email: this._stored('caduceus_email').toLowerCase(),
      role: this._stored('caduceus_role'),
      domains: this._list(this._parsed(this._stored('caduceus_domains'))).map(String),
      mailboxNames: /** @type {Record<string, string>} */ (this._record(this._parsed(this._stored('caduceus_mailbox_names')))),
      externalAccounts: this._externalAccounts,
      externalFolders: this._externalFolders,
    }
    /** @type {MailPreferencesChanged} */
    const changed = { use24HourClock: preferences.use24HourClock === true }
    this.signedIn = session.token !== ''
    this.applicationEventPublisher.publishEvent(session, true)
    this.applicationEventPublisher.publishEvent(changed, true)
  }

  /** @param {string} key @returns {string} */
  _stored(key) {
    return sessionStorage.getItem(key) ?? ''
  }

  /** @param {string} text @returns {unknown} */
  _parsed(text) {
    try { return JSON.parse(text) } catch { return null }
  }

  /** @param {unknown} value @returns {unknown[]} */
  _list(value) {
    if (Array.isArray(value)) return value
    return []
  }

  /** @param {unknown} value @returns {Record<string, any>} */
  _record(value) {
    if (typeof value === 'object' && value !== null && !Array.isArray(value)) return /** @type {Record<string, any>} */ (value)
    return {}
  }
}
