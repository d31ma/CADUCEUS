// @ts-check

/**
 * Team page — shows everyone in the same domain(s) as the current user, with
 * online/offline status from the `/presence/team` endpoint.
 *
 * Refreshes on mount and every 20 seconds while the page is open so the
 * online indicators stay reasonably fresh. The poll is cleared automatically
 * when the user navigates away (Tac tears the component down).
 */
export default class {
  static __tachyonOnMount = ['init']

  /** @type {boolean} */
  get canUseApp() { return !!(window._caduceus?.auth?.isLoggedIn || (typeof sessionStorage !== 'undefined' && sessionStorage.getItem('caduceus_token'))) }
  /** @type {boolean} */
  get canShowLogin() { return !this.canUseApp }

  /** @type {Array<Record<string, any>>} */
  $teammates = []
  /** @type {boolean} */
  $loading = true

  /** @type {?ReturnType<typeof setInterval>} */
  _pollTimer = null

  async init() {
    await this.refresh()
    // Light polling — 20s is enough to surface presence changes without
    // hammering the server. The component unmount handler clears it.
    this._pollTimer = setInterval(() => this.refresh(), 20_000)
  }

  /**
   * Pull the latest roster + online state from the server.
   * @async
   */
  async refresh() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$loading = false; return }
    try {
      const res = await apiFetch('/presence/team')
      this.$teammates = res?.ok ? await res.json() : []
    } catch {
      this.$teammates = []
    } finally {
      this.$loading = false
    }
  }

  /**
   * Two-character avatar initials, falling back to the email local-part
   * if the user has no display name.
   * @param {Record<string, any>} member
   * @returns {string}
   */
  initials(member) {
    const src = (member?.name || member?.email || '').trim()
    if (!src) return '?'
    const parts = src.split(/[@\s._-]+/).filter(Boolean)
    if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
    return src.slice(0, 2).toUpperCase()
  }

  /**
   * Format the right-aligned status label: "Online" or a relative
   * "Last seen N min ago".
   * @param {Record<string, any>} member
   * @returns {string}
   */
  statusLabel(member) {
    if (member?.online) return 'Online'
    if (!member?.lastSeenAt) return 'Offline'
    const diff = Date.now() - Date.parse(member.lastSeenAt)
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return 'Just now'
    if (mins < 60) return `${mins} min ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    return `${days}d ago`
  }
}
