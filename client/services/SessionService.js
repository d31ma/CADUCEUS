import { restoreMailPreferences } from '../shared/scripts/mail-preferences.js'

/**
 * Minimal local stand-in for the shell's SessionService while the shell is
 * ported in parallel. It reads the sign-in that the login flow keeps in
 * sessionStorage and announces it, and the user's mail preferences, to the
 * Go reader services as retained events.
 */
@Service
export default class SessionService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  signedIn = false

  @OnStart
  start() {
    const token = this._stored('caduceus_token')
    this.signedIn = token !== ''
    this._announce(token, this._stored('caduceus_email'))
  }

  /**
   * @param {string} token
   * @param {string} email
   */
  _announce(token, email) {
    /** @type {SessionChanged} */
    const session = { token, email }
    this.applicationEventPublisher.publishEvent(session, true)
    const preferences = restoreMailPreferences()
    /** @type {MailPreferencesChanged} */
    const changed = {
      markAsReadDelay: preferences.markAsReadDelay,
      detailedHeaders: preferences.detailedHeaders,
      detailedNames: preferences.detailedNames,
      conversationOrder: preferences.conversationOrder,
      emailRenderMode: preferences.emailRenderMode,
      defaultReplyAction: preferences.defaultReplyAction,
      displayAttachmentPreviews: preferences.displayAttachmentPreviews,
    }
    this.applicationEventPublisher.publishEvent(changed, true)
  }

  /** @param {string} key @returns {string} */
  _stored(key) {
    return String(sessionStorage.getItem(key) ?? '')
  }
}
