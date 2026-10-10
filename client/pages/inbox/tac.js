// @ts-check

/**
 * Inbox page.
 *
 * Primary email list view. Renders the inbox email list and supports
 * single-email selection. On narrow viewports (mobile) selecting an email
 * navigates directly to the email detail page instead of using the split-pane
 * layout.
 */
export default class {
  static __tachyonOnMount = ['bindMailSignals']

  /** @type {string} */
  selectedMail = ''

  /**
   * Whether the current user is authenticated and the app can be used.
   * @type {boolean}
   */
  get canUseApp() { return !!(window._caduceus?.auth?.isLoggedIn || (typeof sessionStorage !== 'undefined' && sessionStorage.getItem('caduceus_token'))) }

  /**
   * Whether the login prompt should be shown (app is loaded but user is not
   * authenticated).
   * @type {boolean}
   */
  get canShowLogin() { return !this.canUseApp }

  /**
   * Handles the mail-select signal published by the inbox list when a row is
   * clicked. Tachyon 26.28 replaced component `emit` + parent `on:` bindings
   * with the publish/subscribe signal bus, so selection now arrives as a signal
   * value (the email ID) rather than a DOM CustomEvent. On mobile viewports
   * navigates directly to the email detail page.
   * @param {string} id - The selected email ID.
   */
  mailSelected(id) {
    if (id) this.selectedMail = id
    if (this.selectedMail) this.notifyEmailSelected(this.selectedMail)
    // On mobile, navigate to email detail page
    if (window.innerWidth <= 760 && this.selectedMail) {
      window._caduceus?.navigate(`/email/${encodeURIComponent(this.selectedMail)}`)
      return
    }
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /**
   * Tell an already-mounted split-pane detail view to load the new message.
   *
   * Tachyon keeps the child component instance alive while the parent rerenders,
   * so relying only on the first mount leaves the reader on the first email.
   * @param {string} emailId
   */
  notifyEmailSelected(emailId) {
    queueMicrotask(() => {
      window.dispatchEvent(new CustomEvent('caduceus:email-selected', { detail: { emailId } }))
    })
  }

  /**
   * Clears the currently selected mail ID (published by the detail view's Back
   * button via the `mail-back` signal).
   */
  mailDeselected() {
    this.selectedMail = ''
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /** Bind the cross-component mailbox signals using browser-native events. */
  bindMailSignals() {
    window.removeEventListener('caduceus:mail-select', /** @type {EventListener} */ (window._caduceusInboxMailSelect))
    window.removeEventListener('caduceus:mail-back', /** @type {EventListener} */ (window._caduceusInboxMailBack))
    window._caduceusInboxMailSelect = (event) => {
      this.mailSelected(/** @type {CustomEvent<{ emailId?: string }>} */ (event).detail?.emailId || '')
    }
    window._caduceusInboxMailBack = () => this.mailDeselected()
    window.addEventListener('caduceus:mail-select', window._caduceusInboxMailSelect)
    window.addEventListener('caduceus:mail-back', window._caduceusInboxMailBack)
  }

}
