/**
 * Browser work of the mailbox pages that their WebAssembly services cannot
 * do: leaving the page (open a message, open the composer), a confirm()
 * question, and the activity "last viewed" mark that the shell reads from
 * localStorage. The services ask for it with events.
 */
@Service
export default class MailNavigationService {
  static _PREFILL = 'caduceus_compose_prefill'
  static _NARROW = 760

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  /** @param {Event} _event @param {string} emailId */
  openEmail(_event, emailId) {
    this._openWhenPresent(emailId)
  }

  /** @param {Event} _event */
  compose(_event) {
    this._compose({})
  }

  /** @param {Event} _event @param {string} question @param {string} action @param {string} id */
  confirmThen(_event, question, action, id) {
    this._publishWhen(confirm(question), { action, id })
  }

  /** @param {ComposeRequested} prefill */
  @OnEvent
  composeRequested(prefill) {
    this._compose(prefill)
  }

  /** @param {EmailOpenRequested} request */
  @OnEvent
  emailOpenRequested(request) {
    this._go(this._emailPath(request.emailId))
  }

  /** On a phone-sized viewport an opened row leaves for the message page. @param {MailSelected} selected */
  @OnEvent
  mailSelected(selected) {
    this._goWhenNarrow(this._emailPath(selected.emailId))
  }

  /** @param {ActivityViewed} viewed */
  @OnEvent
  activityViewed(viewed) {
    this._remember(`caduceus_activity_viewed:${viewed.owner}`, viewed.timestamp)
    dispatchEvent(new CustomEvent('caduceus:activity-viewed', { detail: { timestamp: viewed.timestamp } }))
  }

  /** @param {Record<string, unknown>} prefill */
  _compose(prefill) {
    this._storePrefill(JSON.stringify(prefill), Object.keys(prefill).length > 0)
    this._go('/compose')
  }

  /** @param {string} text @param {boolean} present */
  _storePrefill(text, present) {
    if (present) sessionStorage.setItem(MailNavigationService._PREFILL, text)
    else sessionStorage.removeItem(MailNavigationService._PREFILL)
  }

  /** @param {boolean} confirmed @param {ConfirmationGiven} confirmation */
  _publishWhen(confirmed, confirmation) {
    if (confirmed) this.applicationEventPublisher.publishEvent(confirmation, false)
  }

  /** @param {string} path */
  _goWhenNarrow(path) {
    if (innerWidth <= MailNavigationService._NARROW) this._go(path)
  }

  /** @param {string} emailId */
  _openWhenPresent(emailId) {
    if (emailId) this._go(this._emailPath(emailId))
  }

  /** @param {string} key @param {string} value */
  _remember(key, value) {
    try { localStorage.setItem(key, value) } catch {}
  }

  /** @param {string} emailId @returns {string} */
  _emailPath(emailId) {
    return `/email/${encodeURIComponent(emailId)}`
  }

  /** @param {string} path */
  _go(path) {
    location.assign(path)
  }
}
