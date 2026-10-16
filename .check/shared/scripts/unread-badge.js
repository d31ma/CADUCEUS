// @ts-check

const TITLE_PREFIX = /^\(\d+\)\s+/
const DEFAULT_TITLE = 'CADUCEUS'

/**
 * Convert an untrusted unread value into the non-negative integer accepted by
 * the Badging API. Mailbox snapshots are authoritative, so fractional,
 * negative, and non-numeric values never leak into platform state.
 *
 * @param {unknown} value
 */
export function normalizeUnreadCount(value) {
  const count = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(count) ? Math.max(0, Math.trunc(count)) : 0
}

/** @param {unknown} values */
function normalizeDomains(values) {
  if (!Array.isArray(values)) return []
  return [...new Set(values
    .map(value => String(value || '').trim().toLowerCase())
    .filter(Boolean))]
}

/**
 * Normalize server or event preference shapes against the authenticated
 * account list. A removed/foreign focused domain safely becomes aggregate.
 *
 * @param {unknown} value
 * @param {unknown} domains
 * @returns {{ scope: 'all'|'domain', domain: string }}
 */
export function normalizeUnreadBadgePreference(value, domains) {
  const input = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, unknown>} */ (value)
    : {}
  const allowed = normalizeDomains(domains)
  const rawDomain = input.domain ?? input.unreadBadgeDomain
  const requestedDomain = typeof rawDomain === 'string' ? rawDomain.trim().toLowerCase() : ''
  const domain = allowed.includes(requestedDomain) ? requestedDomain : allowed[0] || ''
  const rawScope = input.scope ?? input.unreadBadgeScope
  const scope = rawScope === 'domain' && requestedDomain === domain ? 'domain' : 'all'
  return { scope, domain }
}

/**
 * @param {unknown} snapshot
 * @param {{ scope?: unknown, domain?: unknown }} preference
 */
export function resolveUnreadBadgeCount(snapshot, preference) {
  const counts = snapshot && typeof snapshot === 'object' && !Array.isArray(snapshot)
    ? /** @type {Record<string, any>} */ (snapshot)
    : {}
  if (preference?.scope !== 'domain') return normalizeUnreadCount(counts.unreadInbox)
  const domain = typeof preference.domain === 'string' ? preference.domain.trim().toLowerCase() : ''
  const byDomain = counts.unreadInboxByDomain && typeof counts.unreadInboxByDomain === 'object'
    ? counts.unreadInboxByDomain
    : {}
  return normalizeUnreadCount(byDomain[domain])
}

/** @param {string} title */
export function stripUnreadTitle(title) {
  return String(title || '').replace(TITLE_PREFIX, '')
}

/** @param {string} title @param {unknown} count */
export function formatUnreadTitle(title, count) {
  const base = stripUnreadTitle(title).trim() || DEFAULT_TITLE
  const unread = normalizeUnreadCount(count)
  return unread > 0 ? `(${unread}) ${base}` : base
}

/**
 * Synchronizes unread inbox state with the installed-app badge. Browsers that
 * do not expose the Badging API receive an equivalent, privacy-aware tab-title
 * prefix. The controller is intentionally fed by complete mailbox snapshots;
 * it never guesses by incrementing a locally cached count.
 */
export class UnreadBadgeController {
  /**
   * @param {{
   *   windowObject?: Window,
   *   documentObject?: Document,
   *   navigatorObject?: Navigator,
   * }} [options]
   */
  constructor(options = {}) {
    this.windowObject = options.windowObject ?? window
    this.documentObject = options.documentObject ?? document
    this.navigatorObject = options.navigatorObject ?? navigator
    this.count = 0
    this.preference = /** @type {{ scope: 'all'|'domain', domain: string }} */ ({ scope: 'all', domain: '' })
    this.folderSnapshot = /** @type {null | { unreadInbox: unknown, unreadInboxByDomain: Record<string, unknown> }} */ (null)
    this.installed = false
    this.pendingDuringScreenshot = false
    this.titleDecorated = false
    this.titleBase = DEFAULT_TITLE
    this.lastRenderedTitle = ''
    this.revision = 0
    this._onFolderCounts = (event) => {
      const detail = /** @type {CustomEvent<any>} */ (event).detail
      if (!detail || typeof detail !== 'object' || !('unreadInbox' in detail)) return
      this.folderSnapshot = {
        unreadInbox: detail.unreadInbox,
        unreadInboxByDomain: detail.unreadInboxByDomain && typeof detail.unreadInboxByDomain === 'object'
          ? detail.unreadInboxByDomain
          : {},
      }
      void this.set(resolveUnreadBadgeCount(this.folderSnapshot, this.preference))
    }
    this._onUnreadCount = (event) => {
      const detail = /** @type {CustomEvent<any>} */ (event).detail
      void this.set(detail && typeof detail === 'object' && 'count' in detail ? detail.count : detail)
    }
    this._onScreenshotMode = (event) => {
      const enabled = Boolean(/** @type {CustomEvent<any>} */ (event).detail?.enabled)
      if (!enabled && this.pendingDuringScreenshot) {
        this.pendingDuringScreenshot = false
        this._renderTitleFallback()
      }
    }
    this._onPreference = (event) => {
      const detail = /** @type {CustomEvent<any>} */ (event).detail
      if (!detail || typeof detail !== 'object') return
      void this.setPreference(detail, detail.domains)
    }
  }

  install() {
    if (this.installed) return this
    this.installed = true
    this.windowObject.addEventListener('caduceus:folder-counts', this._onFolderCounts)
    this.windowObject.addEventListener('caduceus:unread-count', this._onUnreadCount)
    this.windowObject.addEventListener('caduceus:unread-badge-preference', this._onPreference)
    this.windowObject.addEventListener('caduceus:screenshot-mode', this._onScreenshotMode)
    return this
  }

  destroy() {
    if (!this.installed) return
    this.installed = false
    this.windowObject.removeEventListener('caduceus:folder-counts', this._onFolderCounts)
    this.windowObject.removeEventListener('caduceus:unread-count', this._onUnreadCount)
    this.windowObject.removeEventListener('caduceus:unread-badge-preference', this._onPreference)
    this.windowObject.removeEventListener('caduceus:screenshot-mode', this._onScreenshotMode)
  }

  /** @param {unknown} value */
  async set(value) {
    const revision = ++this.revision
    this.count = normalizeUnreadCount(value)
    this._postToServiceWorker()

    const platform = /** @type {Navigator & { setAppBadge?: (count?: number) => Promise<void>, clearAppBadge?: () => Promise<void> }} */ (this.navigatorObject)
    if (typeof platform.setAppBadge === 'function') {
      try {
        if (this.count > 0) await platform.setAppBadge(this.count)
        else if (typeof platform.clearAppBadge === 'function') await platform.clearAppBadge()
        else await platform.setAppBadge(0)
        if (revision !== this.revision) return
        this.pendingDuringScreenshot = false
        this._restoreDecoratedTitle()
        return
      } catch {
        // A browser can expose the method while the current platform or
        // install state rejects it. Fall through to the tab-title equivalent.
      }
    }

    if (revision !== this.revision) return
    this._renderTitleFallback()
  }

  async clear() {
    await this.set(0)
  }

  /** @param {unknown} value @param {unknown} domains */
  async setPreference(value, domains) {
    this.preference = normalizeUnreadBadgePreference(value, domains)
    if (this.folderSnapshot) {
      await this.set(resolveUnreadBadgeCount(this.folderSnapshot, this.preference))
    }
    return this.preference
  }

  _isScreenshotMode() {
    return this.documentObject.documentElement.getAttribute('data-screenshot-mode') === 'true'
  }

  _renderTitleFallback() {
    if (this._isScreenshotMode()) {
      this.pendingDuringScreenshot = true
      return
    }

    const current = this.documentObject.title || ''
    const base = this.titleDecorated && current === this.lastRenderedTitle
      ? this.titleBase
      : stripUnreadTitle(current).trim() || DEFAULT_TITLE
    this.titleBase = base
    this.lastRenderedTitle = formatUnreadTitle(base, this.count)
    this.titleDecorated = this.count > 0
    this.documentObject.title = this.lastRenderedTitle
  }

  _restoreDecoratedTitle() {
    if (!this.titleDecorated) return
    if (this.documentObject.title === this.lastRenderedTitle) this.documentObject.title = this.titleBase
    this.titleDecorated = false
    this.lastRenderedTitle = ''
  }

  _postToServiceWorker() {
    try {
      this.navigatorObject.serviceWorker?.controller?.postMessage({
        type: 'caduceus:set-badge',
        count: this.count,
      })
    } catch {
      // The foreground Badging API or title fallback remains authoritative.
    }
  }
}

/** @returns {UnreadBadgeController | null} */
export function installUnreadBadge() {
  if (typeof window === 'undefined' || typeof document === 'undefined' || typeof navigator === 'undefined') return null
  if (!document.title.trim()) document.title = DEFAULT_TITLE
  if (window._caduceusUnreadBadge) return window._caduceusUnreadBadge
  const controller = new UnreadBadgeController().install()
  window._caduceusUnreadBadge = controller
  return controller
}

installUnreadBadge()
