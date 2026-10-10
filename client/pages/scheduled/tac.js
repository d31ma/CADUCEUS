// @ts-check

import { formatPreferenceDate } from '../../shared/scripts/mail-preferences.js'

/**
 * Scheduled emails page.
 *
 * Lists emails that have been scheduled for future delivery. On mount it
 * fetches the scheduled-send queue from the API and renders each item with a
 * cancel action. Exposes auth-guard getters for the app shell.
 */
export default class {
  static __tachyonOnMount = ['init']

  /** @type {Array<{ id: string, [key: string]: any }>} */
  $items = []
  /** @type {boolean} */
  @SessionState
  loading = true

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
   * Lifecycle hook — fetches the scheduled email list on mount.
   *
   * @async
   * @returns {Promise<void>}
   */
  async init() {
    await this.load()
  }

  /**
   * Fetches the list of scheduled (future-dated) emails from the API.
   *
   * @async
   * @returns {Promise<void>}
   */
  async load() {
    this.loading = true
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) { this.loading = false; return }
    try {
      const res = await apiFetch('/send/scheduled')
      if (res?.ok) {
        const items = await res.json()
        this.$items = Array.isArray(items) ? items.map(item => ({
          ...item,
          recipientLabel: Array.isArray(item.to) ? item.to.join(', ') : String(item.to || ''),
          sendAtLabel: formatPreferenceDate(item.sendAt),
          statusLabel: item.status === 'sending'
            ? 'Sending now'
            : item.attempts > 0
              ? `Retry ${item.attempts} · ${item.nextAttempt ? formatPreferenceDate(item.nextAttempt) : 'pending'}`
              : 'Queued',
        })) : []
      }
    } catch { this.$items = [] }
    finally { this.loading = false }
  }

  /** Load the full exact-owner message and hand it to the compose route. */
  async editScheduled(_event, id) {
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    try {
      const res = await apiFetch(`/send/scheduled/${id}`)
      if (!res?.ok) {
        window._caduceus?.toast('This scheduled message is no longer editable.')
        await this.load()
        return
      }
      const item = await res.json()
      window._caduceus?.compose({
        scheduledId: item.id,
        sendAt: item.sendAt,
        remindAt: item.remindAt || '',
        from: item.from,
        to: Array.isArray(item.to) ? item.to.join(', ') : String(item.to || ''),
        cc: Array.isArray(item.cc) ? item.cc.join(', ') : String(item.cc || ''),
        replyTo: Array.isArray(item.replyTo) ? item.replyTo[0] || '' : String(item.replyTo || ''),
        subject: item.subject,
        body: item.html || item.text || '',
        inReplyTo: item.inReplyTo || '',
        references: item.references || [],
        attachments: item.attachments || [],
        openTracking: item.openTracking === true,
        linkTracking: item.linkTracking === true,
      })
    } catch {
      window._caduceus?.toast('Network error.')
    }
  }

  /**
   * Cancels a scheduled send after user confirmation.
   *
   * Sends a DELETE request to the API and removes the item from the local
   * list on success.
   *
   * @async
   * @param {string} id - The ID of the scheduled email to cancel.
   * @returns {Promise<void>}
   */
  async cancelScheduled(_event, id) {
    if (!confirm('Cancel this scheduled send?')) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    try {
      const res = await apiFetch(`/send/scheduled/${id}`, { method: 'DELETE' })
      if (res?.ok) {
        window._caduceus?.toast('Scheduled send cancelled.')
        this.$items = this.$items.filter(item => item.id !== id)
      } else {
        const data = await res?.json().catch(() => ({}))
        window._caduceus?.toast(data?.conflict ? 'The message is already being delivered.' : 'Failed to cancel.')
      }
    } catch {
      window._caduceus?.toast('Network error.')
    }
  }
}
