// @ts-check

/**
 * Folder view page (parameterised by folder name).
 *
 * Displays the email list for an arbitrary folder identified by the `_name`
 * route parameter. Supports single-email selection (read-only preview in the
 * detail pane) and exposes auth-guard getters for the app shell.
 */
export default class {
  static __tachyonOnMount = ['bindMailSignals']

  /** @type {string} */
  selectedMail = ''
  /** @type {string} Route-bound folder name. */
  name = ''
  /** @type {string} Explicit folder prop fallback. */
  folder = ''

  /**
   * The current folder name, derived from the route parameter.
   * Falls back to the LAST path segment or 'inbox'.
   *
   * @description The previous `split('/')[1]` returned the literal segment
   * `"folder"` (from `/folder/drafts`) instead of the dynamic param. We
   * now read `.pop()` so `/folder/drafts` → `"drafts"`.
   * @type {string}
   */
  get folderName() {
    return this.name || this.folder || location.pathname.split('/').pop() || 'inbox'
  }

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
   * Handles the mail-select signal published by the email list when a row is
   * clicked (Tachyon 26.28 publish/subscribe bus; the value is the email ID).
   * @param {string} id - The selected email ID.
   */
  mailSelected(id) {
    if (id) this.selectedMail = id
    if (this.selectedMail) this.notifyEmailSelected(this.selectedMail)
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /**
   * Tell an already-mounted split-pane detail view to load the new message.
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
    window.removeEventListener('caduceus:mail-select', /** @type {EventListener} */ (window._caduceusFolderMailSelect))
    window.removeEventListener('caduceus:mail-back', /** @type {EventListener} */ (window._caduceusFolderMailBack))
    window._caduceusFolderMailSelect = (event) => {
      this.mailSelected(/** @type {CustomEvent<{ emailId?: string }>} */ (event).detail?.emailId || '')
    }
    window._caduceusFolderMailBack = () => this.mailDeselected()
    window.addEventListener('caduceus:mail-select', window._caduceusFolderMailSelect)
    window.addEventListener('caduceus:mail-back', window._caduceusFolderMailBack)
  }

}
