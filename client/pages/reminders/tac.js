// @ts-check

import { formatPreferenceDate } from '../../shared/scripts/mail-preferences.js'

/** Follow-up reminders that resurface sent conversations when nobody replies. */
export default class {
  static __tachyonOnMount = ['init']

  /** @type {Array<Record<string, any>>} */
  $items = []
  $loading = true

  get canUseApp() {
    return !!(window._caduceus?.auth?.isLoggedIn
      || (typeof sessionStorage !== 'undefined' && sessionStorage.getItem('caduceus_token')))
  }

  get canShowLogin() { return !this.canUseApp }

  async init() { await this.load() }

  async load() {
    this.$loading = true
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$loading = false; return }
    try {
      const response = await apiFetch('/reminders')
      const reminders = response?.ok ? await response.json() : []
      this.$items = Array.isArray(reminders) ? reminders.map(item => ({
        ...item,
        recipientLabel: item.recipient || '',
        remindAtLabel: formatPreferenceDate(item.remindAt),
      })) : []
    } catch {
      this.$items = []
    } finally {
      this.$loading = false
    }
  }

  openReminder(_event, item) {
    window._caduceus?.openEmailId(item.noticeEmailId || item.emailId)
  }

  async dismissReminder(_event, id) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const response = await apiFetch('/reminders', {
        method: 'DELETE', body: JSON.stringify({ id }),
      })
      if (!response?.ok) throw new Error('dismiss failed')
      this.$items = this.$items.filter(item => item.id !== id)
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      window._caduceus?.toast('Reminder dismissed.')
    } catch {
      window._caduceus?.toast('Could not dismiss reminder.')
    }
  }
}
