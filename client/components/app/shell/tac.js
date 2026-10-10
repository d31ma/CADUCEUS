// @ts-check

import '../../../shared/scripts/imports.js'
import {
  isScreenshotModeEnabled,
  restoreScreenshotMode,
  setScreenshotMode,
} from '../../../shared/scripts/screenshot-mode.js'
import {
  clearFontPreferences,
  persistFontPreferences,
  restoreFontPreferences,
} from '../../../shared/scripts/font-preferences.js'
import {
  clearNotificationSounds,
  persistNotificationSounds,
  playNotificationSound,
  restoreNotificationSounds,
} from '../../../shared/scripts/notification-sounds.js'
import {
  DEFAULT_THEME_PREFERENCES,
  normalizeThemePreferences,
  persistThemePreferences,
  resolveTheme,
  restoreThemePreferences,
  themeBaseMode,
  watchSystemTheme,
} from '../../../shared/scripts/theme-preferences.js'
import { normalizeUnreadBadgePreference } from '../../../shared/scripts/unread-badge.js'
import { clearTrustedExtensions, loadTrustedExtensions } from '../../../shared/scripts/extension-runtime.js'
import { composeLaunchPrefill, isManifestComposeLaunch } from '../../../shared/scripts/pwa-launch.js'
import {
  clearOfflineWriteVault,
  flushOfflineWrites,
  migrateLegacyOfflineMailActions,
  queueOfflineWrite,
} from '../../../shared/scripts/offline-write-vault.js'
import {
  cacheOfflineMailboxSummaries,
  hydrateOfflineMailbox,
} from '../../../shared/scripts/offline-mailbox-index.js'
import {
  buildSystemNotifications,
  highestPriorityNotification,
  visibleNotifications,
} from '../../../shared/scripts/notification-center.js'
import {
  applyMailPreferences,
  clearMailPreferences,
  persistMailPreferences,
  restoreMailPreferences,
} from '../../../shared/scripts/mail-preferences.js'
import { normalizeExternalAccounts } from '../../../shared/scripts/external-accounts-ui.js'
import {
  externalFolderActions,
  externalFolderOpenKey,
  normalizeExternalFolders,
  visibleExternalFolderRows,
} from '../../../shared/scripts/external-folder-tree.js'
import {
  currentInterfaceLocale,
  currentInterfaceLocaleChoice,
  formatDate as formatLocalizedDate,
  formatNumber as formatLocalizedNumber,
  normalizeInterfaceLocale,
  persistInterfaceLocale,
  restoreInterfaceLocale,
  translate,
} from '../../../shared/scripts/localization.js'

// Apply session-cached typography before the shell's first rendered frame.
// The authenticated preferences request revalidates it after mount.
const initialFontPreferences = restoreFontPreferences()
const initialNotificationSounds = restoreNotificationSounds()
const initialThemePreferences = restoreThemePreferences()
const initialMailPreferences = restoreMailPreferences()
const initialInterfaceLocale = restoreInterfaceLocale()
applyMailPreferences(initialMailPreferences)

/**
 * Main application shell component.
 *
 * Serves as the root layout and global state manager for the CADUCEUS email
 * client. On mount it restores the authentication session from session storage,
 * registers the global `window._caduceus` API surface, binds keyboard shortcuts
 * and navigation event listeners, and manages toast notifications, theme
 * toggling, sidebar state, drag-and-drop folder moves, and the compose
 * prefill pipeline.
 *
 * When the user is authenticated and visiting the root path `/` they are
 * automatically redirected to `/inbox`.
 */
export default class {
  static __tachyonOnMount = ['init']

  /** @type {?string} */
  $token = null
  /** @type {?string} */
  $email = null
  /** @type {?string} */
  $name = null
  /** @type {?string} */
  $role = null
  /** @type {string[]} */
  $domains = []
  /** @type {Record<string, string>} Personal display labels keyed by real mailbox domain. */
  $mailboxNames = {}
  /** @type {Array<Record<string, any>>} Owner-scoped connected mailboxes. */
  $externalAccounts = []
  /** @type {Record<string, Array<Record<string, string>>>} Provider folders keyed by account ID. */
  $externalFolders = {}
  /** @type {string} Catalog-backed application font identifier. */
  $interfaceFont = initialFontPreferences.interfaceFont
  /** @type {string} Catalog-backed rich-composer font identifier. */
  $composerFont = initialFontPreferences.composerFont
  /** @type {ReturnType<typeof normalizeInterfaceLocale>} Server-synchronized interface locale choice. */
  $interfaceLocale = initialInterfaceLocale.choice
  /** @type {Record<'new-mail'|'hit-send'|'send'|'confirm', 'none'|'chime'|'bell'|'pop'|'pulse'>} */
  $notificationSounds = initialNotificationSounds
  /** @type {{ activeTheme: 'automatic'|'light'|'dark'|'darkside'|'less-is-more'|'taiga'|'ubuntu', lightTheme: 'light'|'less-is-more'|'taiga'|'ubuntu', darkTheme: 'dark'|'darkside' }} */
  $themePreferences = normalizeThemePreferences(initialThemePreferences)
  /** @type {ReturnType<typeof restoreMailPreferences>} */
  $mailPreferences = initialMailPreferences
  /** @type {string} */
  @SessionState
  toastMsg = ''
  /** @type {boolean} */
  @SessionState
  toastVisible = false
  /** @type {string} */
  @SessionState
  toastActionLabel = ''
  /** @type {boolean} Whether the accessible keyboard-command dialog is open. */
  @SessionState
  shortcutHelpOpen = false
  /** @type {boolean} Whether the connected-folder mutation dialog is open. */
  @SessionState
  externalFolderDialogOpen = false
  /** @type {'create'|'rename'|'delete'} */
  @SessionState
  externalFolderDialogMode = 'create'
  @SessionState
  externalFolderDialogName = ''
  @SessionState
  externalFolderDialogError = ''
  @SessionState
  externalFolderDialogBusy = false
  /** @type {boolean} Whether the full prioritized notification panel is open. */
  notificationCenterOpen = false
  /** @type {Array<Record<string, any>>} */
  @SessionState
  notifications = []
  /** @type {string[]} User-scoped dismissals for optional derived notices. */
  @SessionState
  dismissedNotifications = []
  /** @type {{online:boolean,updateVersion:string,accountErrors:string[],disabledRuleCount:number,pushPrompt:boolean}} */
  @SessionState
  notificationState = { online: true, updateVersion: '', accountErrors: [], disabledRuleCount: 0, pushPrompt: false }
  /** @type {boolean} */
  @SessionState
  isRoot = true
  /** @type {string} */
  @SessionState
  activeFolder = 'inbox'
  /**
   * Domain selected by the current URL's `?domain=` query string, or empty
   * when the user is on an aggregate view ("All Inboxes" / "All Sent").
   * Sidebar highlight logic uses this to disambiguate the Favorites row from
   * a per-domain row that matches the same folder name.
   * @type {string}
   */
  @SessionState
  selectedDomain = ''
  /** @type {string} Connected-account ID selected by the current URL. */
  @SessionState
  selectedAccount = ''
  /** @type {boolean} Whether the topbar user-menu dropdown is currently open. */
  userMenuOpen = false
  /** @type {boolean} Whether the responsive sidebar drawer is open. */
  mobileSidebarOpen = false
  /** @type {boolean} Whether all visible personal data is masked for screenshots. */
  $screenshotMode = false
  /** @type {Record<string, number>} Per-folder email counts shown in the sidebar (legacy aggregate). */
  folderCounts = {}
  /**
   * Per-domain, per-folder email counts. Shape: `{ [domain]: { [folder]: count } }`.
   * Populated alongside {@link folderCounts} so per-domain sidebar rows can
   * render accurate badges without an extra round-trip.
   * @type {Record<string, Record<string, number>>}
   */
  folderCountsByDomain = {}
  /** @type {Record<string, Record<string, number>>} Per-connected-account folder counts. */
  folderCountsByAccount = {}
  /**
   * User's chosen accent color (key into {@link ACCENT_PRESETS}). Persisted
   * to localStorage via the `$$` prefix so the choice survives reloads and
   * lives independently of session login state.
   * @type {string}
   */
  $$accentColor = 'teal'

  /** @type {?ReturnType<typeof setTimeout>} */
  _toastTimer = null
  /** @type {?(() => unknown)} */
  _toastAction = null
  /** @type {HTMLElement | null} Focus target restored after shortcut help closes. */
  _shortcutHelpReturnFocus = null
  /** @type {HTMLElement | null} */
  _externalFolderDialogReturnFocus = null
  _externalFolderDialogAccountId = ''
  _externalFolderDialogFolderId = ''
  /** @type {HTMLElement | null} Focus target restored after notification center closes. */
  _notificationCenterReturnFocus = null
  /** @type {{ id: string, tagName: string, ariaLabel: string, href: string, name: string, type: string } | null} */
  _shortcutHelpReturnIdentity = null
  /** @type {?ReturnType<typeof setInterval>} */
  _activityTimer = null
  /** @type {Promise<void> | null} */
  _offlineMailboxSync = null
  /** @type {null | (() => void)} */
  _stopThemeWatcher = null
  /**
   * Sidebar section open/closed state — reactive so `:class` bindings on the
   * sections re-evaluate when the user toggles. Persisted via `$$` so the
   * choice survives reloads. Keys are the section identifiers
   * (`mailboxes`, `folders`).
   * @type {{ mailboxes: boolean, folders: boolean }}
   */
  @LocalState
  sidebarOpen = { mailboxes: true, folders: true }

  /**
   * Open/closed state for the collapsible Favorites groups. Mirrors Apple
   * Mail: "All Inboxes" and "All Sent" each disclose per-domain children
   * via a chevron. Persisted to localStorage via the `$$` prefix so the
   * user's preference survives reloads.
   * @type {{ inboxes: boolean, sent: boolean }}
   */
  @LocalState
  favoritesOpen = { inboxes: true, sent: false }

  /**
   * Expanded connected-account folders keyed by account plus stable provider
   * identifier. The persisted key cannot collide when two accounts expose the
   * same IMAP path.
   * @type {Record<string, boolean>}
   */
  @LocalState
  externalFolderOpen = {}

  /** @type {boolean} Explicit template-safe authentication state. */
  isAuthenticated = false

  /** @type {boolean} Explicit template-safe root-route state. */
  isRootPath = true

  /**
   * The authenticated user's email address, or empty string.
   * @type {string}
   */
  get email() { return this.$email || '' }

  /**
   * Friendly display name for the authenticated user.
   *
   * Prefers the explicit `$name` (set from the auth response when the user
   * record has one). Falls back to a humanised version of the email's
   * local-part — `admin@caduceus.com` → `Admin`, `jane.doe@x.com` → `Jane Doe`.
   * Returns an empty string when no user is logged in.
   *
   * @type {string}
   */
  get displayName() {
    if (this.$name && this.$name.trim()) return this.$name.trim()
    const local = (this.$email || '').split('@')[0]
    if (!local) return ''
    // Split on common name-part separators and title-case each piece.
    return local
      .replace(/[._-]+/g, ' ')
      .split(/\s+/)
      .filter(Boolean)
      .map(part => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
      .join(' ')
  }

  /** Translate one fixed UI catalog message in the active interface locale. */
  t(id, values = {}) { return translate(id, values) }

  /** @type {string} Template-safe deployment brand asset. */
  brandLogo = '/shared/assets/favicon.svg'

  /**
   * Initialises the application shell on mount.
   *
   * Restores session state, determines the current route, updates the active
   * folder highlight, restores sidebar collapse state, registers the global
   * `window._caduceus` API, and binds all event listeners.
   *
   * @returns {void}
   */
  init() {
    const ss = sessionStorage
    this.brandLogo = window.CADUCEUS_CONFIG?.brandLogo || '/shared/assets/favicon.svg'
    this.$token   = ss.getItem('caduceus_token') ?? null
    this.$email   = ss.getItem('caduceus_email') ?? null
    this.$name    = ss.getItem('caduceus_name') ?? null
    this.$role    = ss.getItem('caduceus_role') ?? null
    this.isAuthenticated = Boolean(this.$token)
    try { this.$domains = JSON.parse(ss.getItem('caduceus_domains') || '[]') } catch { this.$domains = [] }
    try { this.$mailboxNames = JSON.parse(ss.getItem('caduceus_mailbox_names') || '{}') } catch { this.$mailboxNames = {} }
    const fonts = restoreFontPreferences()
    this.$interfaceFont = fonts.interfaceFont
    this.$composerFont = fonts.composerFont
    this.$notificationSounds = restoreNotificationSounds()
    this.$themePreferences = normalizeThemePreferences(restoreThemePreferences())
    this.$mailPreferences = restoreMailPreferences()
    applyMailPreferences(this.$mailPreferences)
    if (!this._stopThemeWatcher) {
      this._stopThemeWatcher = watchSystemTheme(() => {
        if (this.$themePreferences.activeTheme !== 'automatic') return
        this.setThemePreferences(this.$themePreferences)
        void (/** @type {any} */ (globalThis)).__tc_rerender?.()
      })
    }

    // IndexedDB outlives the sessionStorage token, so a returning visitor (new
    // tab, no token) must not keep a prior user's cached reads on disk. Purge
    // the fetch cache whenever we boot without a session.
    if (!this.$token) {
      this._clearCache()
      void clearOfflineWriteVault()
    }

    this.isRoot = location.pathname === '/'
    this.isRootPath = this.isRoot
    if (this.isRoot && this.isAuthenticated) {
      location.replace('/inbox')
    }
    this.updateActiveFolder()
    this.$screenshotMode = this.isAuthenticated
      ? restoreScreenshotMode()
      : setScreenshotMode(false)
    if (!this.isAuthenticated) void window._caduceusUnreadBadge?.clear()

    this.registerGlobals()
    this.restorePendingSnoozeUndo()
    this.restorePendingSendUndo()
    this.bindEvents()
    this.bindNotificationActions()
    this.bindNotificationCenter()
    this.bindPwaLaunchQueue()
    this.bindOfflineMailActions()
    this.restoreColumnWidths()
    this.bindColumnResizers()
    this.applyAccent()
    this.pulseInboxReady()
    // Kick off an initial folder-count fetch so the sidebar badges render
    // correctly on routes like /settings or /compose where inbox-view
    // isn't mounted. The result is published over the same event the
    // bindEvents listener handles.
    if (this.isAuthenticated) {
      window._caduceusPreferencesReady = false
      void loadTrustedExtensions()
      this.refreshFolderCounts()
      this.startActivityPolling()
      this.loadMailboxNames()
      this.loadExternalMailboxes()
      this.loadNotificationSources()
      this.startPresenceHeartbeat()
      this.startMailStream()
      void this.syncOfflineMailbox()
    }
    // Component lifecycle runs after Tachyon's first document render. Refresh
    // once so session-backed shell state is reflected in the rendered layout.
    setTimeout(() => {
      void (/** @type {any} */ (globalThis)).__tc_rerender?.()
      setTimeout(() => this.bootstrapActivePage(), 0)
    }, 0)
  }

  get visibleNotifications() {
    return visibleNotifications(this.notifications, this.dismissedNotifications)
  }

  get highestNotification() { return highestPriorityNotification(this.visibleNotifications) }
  get notificationCount() { return this.visibleNotifications.length }
  get notificationButtonLabel() {
    return this.notificationCount
      ? `Notifications, ${this.notificationCount} active`
      : 'Notifications, none active'
  }

  notificationCardClass(item) {
    return `shell-notification shell-notification-${item?.kind || 'custom'}${item?.id === this.highestNotification?.id ? ' highest-priority' : ''}`
  }

  refreshNotifications() {
    this.notifications = buildSystemNotifications(this.notificationState)
  }

  bindNotificationCenter() {
    this.notificationState = { ...this.notificationState, online: navigator.onLine !== false }
    this.restoreNotificationDismissals()
    this.refreshNotifications()

    const browserWindow = /** @type {any} */ (window)
    const previousOnline = /** @type {EventListener | undefined} */ (browserWindow._caduceusNotificationOnline)
    const previousOffline = /** @type {EventListener | undefined} */ (browserWindow._caduceusNotificationOffline)
    if (previousOnline) window.removeEventListener('online', previousOnline)
    if (previousOffline) window.removeEventListener('offline', previousOffline)
    browserWindow._caduceusNotificationOnline = () => {
      this.notificationState = { ...this.notificationState, online: true }
      this.refreshNotifications()
    }
    browserWindow._caduceusNotificationOffline = () => {
      this.notificationState = { ...this.notificationState, online: false }
      this.refreshNotifications()
    }
    window.addEventListener('online', browserWindow._caduceusNotificationOnline)
    window.addEventListener('offline', browserWindow._caduceusNotificationOffline)

    if ('serviceWorker' in navigator) {
      void navigator.serviceWorker.ready.then(registration => {
        const markUpdate = () => {
          const worker = registration.installing || registration.waiting
          if (!worker || !navigator.serviceWorker.controller) return
          const reveal = () => {
            if (worker.state !== 'installed') return
            this.notificationState = { ...this.notificationState, updateVersion: 'ready' }
            this.refreshNotifications()
          }
          worker.addEventListener('statechange', reveal)
          reveal()
        }
        registration.addEventListener('updatefound', markUpdate)
        markUpdate()
      }).catch(() => {})
    }
  }

  restoreNotificationDismissals() {
    const key = `caduceus-notification-dismissals:${this.$email || 'anonymous'}`
    try {
      const stored = JSON.parse(localStorage.getItem(key) || '[]')
      this.dismissedNotifications = Array.isArray(stored)
        ? stored.filter(value => typeof value === 'string').slice(-100)
        : []
    } catch { this.dismissedNotifications = [] }
  }

  persistNotificationDismissals() {
    const key = `caduceus-notification-dismissals:${this.$email || 'anonymous'}`
    localStorage.setItem(key, JSON.stringify(this.dismissedNotifications.slice(-100)))
  }

  async loadNotificationSources() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const [rulesResponse, subscriptionsResponse] = await Promise.all([
        apiFetch('/rules'), apiFetch('/notifications/subscriptions'),
      ])
      const rules = rulesResponse?.ok ? await rulesResponse.json() : []
      const subscriptions = subscriptionsResponse?.ok ? await subscriptionsResponse.json() : []
      const pushPrompt = !window.__CADUCEUS_DISABLE_SW && 'Notification' in window
        && Notification.permission === 'default' && Array.isArray(subscriptions) && subscriptions.length === 0
      this.notificationState = {
        ...this.notificationState,
        disabledRuleCount: Array.isArray(rules) ? rules.filter(rule => rule?.enabled === false).length : 0,
        pushPrompt,
      }
      this.refreshNotifications()
    } catch {}
  }

  toggleNotificationCenter() {
    if (this.notificationCenterOpen) { this.closeNotificationCenter(); return }
    this._notificationCenterReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    this.notificationCenterOpen = true
    setTimeout(() => /** @type {HTMLButtonElement | null} */ (
      document.querySelector('.app-notification-panel > header button')
    )?.focus(), 0)
  }
  closeNotificationCenter() {
    this.notificationCenterOpen = false
    const target = this._notificationCenterReturnFocus
    this._notificationCenterReturnFocus = null
    setTimeout(() => target?.focus(), 0)
  }
  notificationCenterKeydown(event) {
    if (event?.key === 'Escape') { event.preventDefault(); this.closeNotificationCenter() }
  }

  dismissNotification(eventOrId, requestedId) {
    const id = typeof eventOrId === 'string' ? eventOrId : requestedId
    const notice = this.visibleNotifications.find(item => item.id === id)
    if (!notice?.dismissable) return
    this.dismissedNotifications = [...new Set([...this.dismissedNotifications, id])].slice(-100)
    this.persistNotificationDismissals()
    this.refreshNotifications()
  }

  runCenterAction(eventOrAction, requestedAction) {
    const action = typeof eventOrAction === 'string' ? eventOrAction : requestedAction
    if (action === 'try-now' || action === 'retry-accounts') {
      this.notificationState = { ...this.notificationState, online: navigator.onLine !== false, accountErrors: [] }
      this.refreshNotifications()
      this.stopMailStream()
      this.startMailStream()
      void this.refreshFolderCounts()
    } else if (action === 'reload-update') {
      location.reload()
    } else if (action === 'manage-accounts') {
      this.navigate('/settings#mailboxes')
    } else if (action === 'view-rules') {
      this.navigate('/settings#mail-rules')
    } else if (action === 'notification-settings') {
      this.navigate('/settings#notifications')
    }
    this.closeNotificationCenter()
  }

  /** Restore a bounded snooze Undo action after a standalone reader navigation. */
  restorePendingSnoozeUndo() {
    const key = 'caduceus_pending_snooze_undo'
    let pending
    try { pending = JSON.parse(sessionStorage.getItem(key) || 'null') } catch { pending = null }
    if (!pending?.id || pending.owner !== this.$email || pending.expiresAt <= Date.now()) {
      sessionStorage.removeItem(key)
      return
    }
    const remaining = Math.max(250, pending.expiresAt - Date.now())
    this.toastAction(pending.message, {
      label: 'Undo',
      onClick: async () => {
        sessionStorage.removeItem(key)
        const response = await this.apiFetch(`/snooze/${encodeURIComponent(pending.id)}`, { method: 'DELETE' })
        this.toast(response?.ok ? 'Snooze undone.' : 'Could not undo snooze.')
        if (response?.ok) {
          void this.playSound('confirm')
          window.dispatchEvent(new Event('caduceus:refresh-inbox'))
        }
      },
    }, remaining)
    setTimeout(() => sessionStorage.removeItem(key), remaining)
  }

  /** Restore a composer Undo action after client-side route replacement. */
  restorePendingSendUndo() {
    const key = 'caduceus_pending_send_undo'
    let pending
    try { pending = JSON.parse(sessionStorage.getItem(key) || 'null') } catch { pending = null }
    if (!pending?.undoId || pending.owner !== this.$email || pending.expiresAt <= Date.now()) {
      sessionStorage.removeItem(key)
      return
    }
    const remaining = Math.max(250, pending.expiresAt - Date.now())
    this.toastAction(String(pending.label || 'Message sent. Undo'), {
      label: 'Undo',
      onClick: async () => {
        sessionStorage.removeItem(key)
        try {
          const response = await this.apiFetch('/send/undo', {
            method: 'POST', body: JSON.stringify({ undoId: pending.undoId }),
          })
          if (!response?.ok) throw new Error('undo failed')
          void this.playSound('confirm')
          this.toast('Send undone.')
          window.dispatchEvent(new Event('caduceus:refresh-inbox'))
        } catch {
          this.toast('Could not undo send.')
        }
      },
    }, remaining)
    setTimeout(() => sessionStorage.removeItem(key), remaining)
  }

  /**
   * Begin pinging `/presence/heartbeat` on a 30s interval so the server
   * keeps this user marked online for the team view. Pings immediately,
   * then on `visibilitychange` when the tab refocuses (so we don't go
   * "stale" when a tab is backgrounded for a long time then resumed).
   *
   * Idempotent — multiple calls don't stack timers.
   */
  startPresenceHeartbeat() {
    if (this._presenceTimer) return
    const ping = async () => {
      try { await this.apiFetch('/presence/heartbeat', { method: 'POST' }) } catch {}
    }
    ping()
    this._presenceTimer = setInterval(ping, 30_000)
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) ping()
    })
  }

  /**
   * Open the live new-mail stream (SSE `GET /inbox/stream`) and keep it alive
   * across reconnects. Owned by the shell so it runs on every page, not just
   * the inbox.
   *
   * Serverless-aware: the connection is short-lived by design (the server
   * closes it before the platform timeout), so we reconnect with backoff and
   * always pass `?since=<cursor>` — the newest event id we've seen — so any
   * mail that arrived during the gap is replayed immediately on reconnect.
   * Web Push (service worker) covers the fully-backgrounded case.
   *
   * Idempotent.
   */
  startMailStream() {
    if (this._mailStream) return
    const ctrl = new AbortController()
    this._mailStream = ctrl
    // Start slightly in the past so mail that landed between the initial inbox
    // load and this connection is replayed too (deduped client-side by id).
    this._mailCursor = new Date(Date.now() - 60_000).toISOString()
    this._streamMailLoop(ctrl)
  }

  /** Tear down the live mail stream (called on sign-out). */
  stopMailStream() {
    try { this._mailStream?.abort() } catch {}
    this._mailStream = null
  }

  /** @param {AbortController} ctrl */
  async _streamMailLoop(ctrl) {
    let backoff = 1000
    let failures = 0
    while (!ctrl.signal.aborted) {
      try {
        if (!this.$token) return
        const apiUrl = window.CADUCEUS_CONFIG?.apiUrl || ''
        const url = `${apiUrl}/inbox/stream?since=${encodeURIComponent(this._mailCursor ?? '')}`
        const res = await fetch(url, {
          headers: { Authorization: `Bearer ${this.$token}`, Accept: 'text/event-stream' },
          cache: 'no-store',
          signal: ctrl.signal,
        })
        if (!res.ok || !res.body) throw new Error(`stream ${res.status}`)
        failures = 0
        if (this.notificationState.accountErrors.length) {
          this.notificationState = { ...this.notificationState, accountErrors: [] }
          this.refreshNotifications()
        }
        backoff = 1000 // healthy connection — reset backoff
        await this._readMailStream(res.body, ctrl)
      } catch {
        if (ctrl.signal.aborted) return
        failures += 1
        if (failures >= 2) {
          this.notificationState = { ...this.notificationState, accountErrors: [...this.$domains] }
          this.refreshNotifications()
        }
      }
      if (ctrl.signal.aborted) return
      await new Promise(resolve => setTimeout(resolve, backoff))
      backoff = Math.min(backoff * 2, 15_000)
    }
  }

  /** @param {ReadableStream<Uint8Array>} body @param {AbortController} ctrl */
  async _readMailStream(body, ctrl) {
    const reader = body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (!ctrl.signal.aborted) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      let sep
      while ((sep = buffer.indexOf('\n\n')) >= 0) {
        const frame = buffer.slice(0, sep)
        buffer = buffer.slice(sep + 2)
        this._handleMailFrame(frame)
      }
    }
  }

  /** @param {string} frame One raw SSE frame (lines joined by \n). */
  _handleMailFrame(frame) {
    let event = 'message'
    let data = ''
    /** @type {string | null} */
    let id = null
    for (const line of frame.split('\n')) {
      if (!line || line.startsWith(':')) continue // blank / heartbeat comment
      if (line.startsWith('event:')) event = line.slice(6).trim()
      else if (line.startsWith('data:')) data += line.slice(5).trim()
      else if (line.startsWith('id:')) id = line.slice(3).trim()
    }
    if (id) this._mailCursor = id // advance cursor so the next reconnect resumes here

    if (event === 'unauthorized') {
      this.stopMailStream()
      this.signOut()
      return
    }
    if (event !== 'new-email') return // 'connected' / 'reconnect' are informational

    let email
    try { email = JSON.parse(data) } catch { return }
    if (!email?.id) return
    if (document.visibilityState === 'visible') void this.playSound('new-mail')
    // Let the inbox list (if mounted) splice it in + show its "N new messages"
    // banner, and refresh the sidebar badges (the passive off-inbox indicator).
    // No per-message toast — bursts are aggregated by the banner instead.
    window.dispatchEvent(new CustomEvent('caduceus:new-email', { detail: email }))
    this.refreshFolderCounts()
    void this.syncOfflineMailbox()
  }

  /**
   * Refresh the encrypted 500-message browser mirror, then fill missing bodies
   * and bounded attachments in idle slices. No bearer token enters IndexedDB.
   */
  async syncOfflineMailbox() {
    if (this._offlineMailboxSync || !this.$token || !this.$email || navigator.onLine === false) return this._offlineMailboxSync
    const owner = this.$email.toLowerCase()
    this._offlineMailboxSync = (async () => {
      const response = await this.apiFetch('/inbox?limit=500')
      if (!response?.ok || this.$email?.toLowerCase() !== owner) return
      const messages = await response.json()
      if (!Array.isArray(messages)) return
      await cacheOfflineMailboxSummaries(owner, messages, { replace: true })
      while (this.$token && this.$email?.toLowerCase() === owner && navigator.onLine !== false) {
        const result = await hydrateOfflineMailbox(owner, path => this.apiFetch(path), { maxMessages: 20 })
        if (!result.remaining || !result.completed) break
        await new Promise(resolve => {
          if (typeof requestIdleCallback === 'function') requestIdleCallback(() => resolve(undefined), { timeout: 2000 })
          else setTimeout(resolve, 100)
        })
      }
    })().catch(() => {}).finally(() => { this._offlineMailboxSync = null })
    return this._offlineMailboxSync
  }

  /**
   * Fetch /inbox once and tally emails per folder. Publishes the result via
   * the `caduceus:folder-counts` window event so the shell's listener picks
   * it up the same way it picks up updates from inbox-view.
   *
   * @async
   * @returns {Promise<void>}
   */
  async refreshFolderCounts() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const res = await apiFetch('/inbox')
      if (!res?.ok) return
      const emails = await res.json()
		if (Array.isArray(emails)) void cacheOfflineMailboxSummaries(this.$email || '', emails)
      /** @type {Record<string, number>} */
      const counts = {}
      /** @type {Record<string, Record<string, number>>} */
      const byDomain = {}
      /** @type {Record<string, Record<string, number>>} */
      const byAccount = {}
      let unreadInbox = 0
      /** @type {Record<string, number>} */
      const unreadInboxByDomain = {}
      for (const e of emails || []) {
        const folder = (e?.systemFolder || e?.folder || 'inbox')
        const domain = String(e?.domain || '').toLowerCase() || 'unknown'
        const accountId = String(e?.providerAccountId || '')
        counts[folder] = (counts[folder] || 0) + 1
        const scopedCounts = accountId
          ? (byAccount[accountId] ||= {})
          : (byDomain[domain] ||= {})
        scopedCounts[folder] = (scopedCounts[folder] || 0) + 1
        if (folder === 'inbox' && e?.read !== true) {
          unreadInbox++
          if (!accountId) unreadInboxByDomain[domain] = (unreadInboxByDomain[domain] || 0) + 1
        }
      }
      // Upcoming-events count: events whose start is in the future, up to
      // the end of next week. Best-effort; not blocking on failure.
      try {
        const now = new Date()
        const nextWeek = new Date(now); nextWeek.setDate(now.getDate() + 14)
        const evRes = await apiFetch(`/calendar/events?start=${encodeURIComponent(now.toISOString())}&end=${encodeURIComponent(nextWeek.toISOString())}`)
        if (evRes?.ok) {
          const events = await evRes.json()
          counts.calendar = (events || []).length
        }
      } catch { /* calendar count optional */ }
      window.dispatchEvent(new CustomEvent('caduceus:folder-counts', {
        detail: { counts, byDomain, byAccount, unreadInbox, unreadInboxByDomain },
      }))
    } catch {
      // Sidebar counts are best-effort; failures stay silent.
    }
  }

  /**
   * Keep the self-hosted recipient-activity badge current without coupling it
   * to the inbox stream. A first-time browser begins at "now" so historic
   * events do not appear as a surprising wall of unread notifications.
   */
  startActivityPolling() {
    if (this._activityTimer) return
    void this.refreshActivityCount()
    this._activityTimer = setInterval(() => {
      if (!document.hidden) void this.refreshActivityCount()
    }, 60_000)
  }

  stopActivityPolling() {
    if (this._activityTimer) clearInterval(this._activityTimer)
    this._activityTimer = null
  }

  /** Load the number of recipient events recorded since this mailbox last viewed Activity. */
  async refreshActivityCount() {
    const apiFetch = window._caduceus?.apiFetch
    const owner = (this.$email || '').toLowerCase()
    if (!apiFetch || !owner) return
    const key = `caduceus_activity_viewed:${owner}`
    let since = ''
    try {
      since = localStorage.getItem(key) || ''
      if (!since) {
        since = new Date().toISOString()
        localStorage.setItem(key, since)
      }
    } catch { since = new Date().toISOString() }
    try {
      const response = await apiFetch(`/activity?range=all&limit=1&since=${encodeURIComponent(since)}`)
      if (!response?.ok) return
      const data = await response.json()
      const unseen = Math.max(0, Number(data?.unseenCount) || 0)
      this.folderCounts = { ...this.folderCounts, activity: unseen }
    } catch {
      // Notification counts are best-effort; the Activity page remains usable.
    }
  }

  /**
   * Refresh personal mailbox labels without placing them into the auth token.
   *
   * @returns {Promise<void>}
   */
  async loadMailboxNames() {
    try {
      // network-first: always revalidate when the API is reachable, but fall
      // back to the last cached preferences if it's cold/slow/down (serverless).
      const res = await this.apiFetchCached('/users/me/preferences', { cache: 'network-first' })
      if (!res?.ok) return
      const preferences = await res.json()
      this.setMailboxNames(preferences?.mailboxNames)
      this.setFontPreferences(preferences || {})
      this.setNotificationSounds(preferences?.notificationSounds)
      this.setThemePreferences(preferences?.themePreferences)
      this.setInterfaceLocale(preferences?.interfaceLocale)
      this.setUnreadBadgePreference(preferences)
      await this.setKeyboardShortcutOverrides(preferences?.keyboardShortcutOverrides)
      this.setMailPreferences(preferences?.mailPreferences)
    } catch {
      // The cached labels remain usable when preference refresh is unavailable.
    } finally {
      window._caduceusPreferencesReady = true
    }
  }

  /** Load redacted connected-account identities and their owner-scoped folder catalogs. */
  async loadExternalMailboxes() {
    try {
      const response = await this.apiFetch('/api/v1/external-accounts')
      if (!response?.ok) return
      const body = await response.json()
      const accounts = normalizeExternalAccounts(body?.accounts)
      const folderPairs = await Promise.all(accounts.map(async account => {
        if (account.capabilities?.mailRead !== true || account.state === 'deleting') return [account.id, []]
        try {
          const foldersResponse = await this.apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}/folders`)
          const foldersBody = foldersResponse?.ok ? await foldersResponse.json() : null
          return [account.id, normalizeExternalFolders(foldersBody?.folders, account.id)]
        } catch {
          return [account.id, []]
        }
      }))
      this.$externalAccounts = accounts
      this.$externalFolders = Object.fromEntries(folderPairs)
      // Render the shell-owned navigation first. Tachyon may replace a child
      // scope during this pass, so publish only after the current inbox scope
      // and its mailbox listener are in the document.
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      window.dispatchEvent(new CustomEvent('caduceus:external-mailboxes', {
        detail: { accounts: this.$externalAccounts, folders: this.$externalFolders },
      }))
    } catch {
      // The hosted mailbox remains available when a provider catalog is down.
    }
  }

  /** @param {Record<string, any>} account @param {number} direction */
  canMoveExternalAccount(account, direction) {
    const index = this.$externalAccounts.findIndex(candidate => candidate.id === account?.id)
    return index >= 0 && index + direction >= 0 && index + direction < this.$externalAccounts.length
  }

  canMoveExternalAccountUp(account) { return this.canMoveExternalAccount(account, -1) }
  canMoveExternalAccountDown(account) { return this.canMoveExternalAccount(account, 1) }
  moveExternalAccountUp(event, account) { return this.moveExternalAccount(event, account, -1) }
  moveExternalAccountDown(event, account) { return this.moveExternalAccount(event, account, 1) }

  /** @param {Event} event @param {Record<string, any>} account */
  async changeExternalAccountColor(event, account) {
    const target = event.target instanceof HTMLInputElement ? event.target : null
    const color = target?.value || ''
    if (!/^#[0-9a-f]{6}$/i.test(color)) return
    const updated = await this._updateExternalAccountPresentation(account, { color })
    if (updated) this.toast('Account color updated.')
  }

  /** @param {Event} _event @param {Record<string, any>} account */
  async toggleExternalAccountFromSidebar(_event, account) {
    if (this._externalAccountPresentationBusy) return
    const action = account.state === 'paused' ? 'resume' : 'pause'
    this._externalAccountPresentationBusy = true
    try {
      const response = await this.apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}`, {
        method: 'PATCH', body: JSON.stringify({ action, revision: account.revision }),
      })
      const body = await response?.json().catch(() => null)
      if (!response?.ok || !body?.id) throw new Error(body?.error || `Could not ${action} connected account.`)
      this._replaceExternalAccounts([body])
      this.toast(action === 'pause' ? 'Account paused.' : 'Account resumed.')
    } catch (error) {
      await this.loadExternalMailboxes()
      this.toast(error instanceof Error ? error.message : `Could not ${action} connected account.`)
    } finally {
      this._externalAccountPresentationBusy = false
    }
  }

  /** @param {Event} _event @param {Record<string, any>} account */
  async retryExternalAccountFromSidebar(_event, account) {
    if (this._externalAccountPresentationBusy) return
    this._externalAccountPresentationBusy = true
    try {
      const response = await this.apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}/retry`, {
        method: 'POST', body: JSON.stringify({ revision: account.revision }),
      })
      const body = await response?.json().catch(() => null)
      if (!response?.ok || body?.scheduled !== true) throw new Error(body?.error || 'Could not synchronize connected account.')
      this.toast('Account synchronization scheduled.')
    } catch (error) {
      await this.loadExternalMailboxes()
      this.toast(error instanceof Error ? error.message : 'Could not synchronize connected account.')
    } finally {
      this._externalAccountPresentationBusy = false
    }
  }

  /** Swap two adjacent account orders with compensating rollback on failure. */
  async moveExternalAccount(_event, account, direction) {
    if (this._externalAccountPresentationBusy || !this.canMoveExternalAccount(account, direction)) return
    const currentIndex = this.$externalAccounts.findIndex(candidate => candidate.id === account.id)
    const neighbor = this.$externalAccounts[currentIndex + direction]
    this._externalAccountPresentationBusy = true
    try {
      const movedNeighbor = await this._requestExternalAccountPresentation(neighbor, { order: account.order })
      if (!movedNeighbor) throw new Error('Could not reorder connected accounts.')
      const movedAccount = await this._requestExternalAccountPresentation(account, { order: neighbor.order })
      if (!movedAccount) {
        await this._requestExternalAccountPresentation(movedNeighbor, { order: neighbor.order }).catch(() => null)
        throw new Error('Could not reorder connected accounts.')
      }
      this._replaceExternalAccounts([movedNeighbor, movedAccount])
      this.toast('Account order updated.')
    } catch (error) {
      await this.loadExternalMailboxes()
      this.toast(error instanceof Error ? error.message : 'Could not reorder connected accounts.')
    } finally {
      this._externalAccountPresentationBusy = false
    }
  }

  async _updateExternalAccountPresentation(account, update) {
    if (this._externalAccountPresentationBusy) return null
    this._externalAccountPresentationBusy = true
    try {
      const updated = await this._requestExternalAccountPresentation(account, update)
      if (!updated) throw new Error('Could not update connected account.')
      this._replaceExternalAccounts([updated])
      return updated
    } catch (error) {
      await this.loadExternalMailboxes()
      this.toast(error instanceof Error ? error.message : 'Could not update connected account.')
      return null
    } finally {
      this._externalAccountPresentationBusy = false
    }
  }

  async _requestExternalAccountPresentation(account, update) {
    const response = await this.apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}`, {
      method: 'PATCH', body: JSON.stringify({ action: 'update', revision: account.revision, ...update }),
    })
    const body = await response?.json().catch(() => null)
    if (!response?.ok || !body?.id) throw new Error(body?.error || 'Could not update connected account.')
    return body
  }

  _replaceExternalAccounts(updates) {
    const byId = new Map(updates.map(account => [account.id, account]))
    this.$externalAccounts = this.$externalAccounts
      .map(account => byId.get(account.id) || account)
      .sort((left, right) => Number(left.order) - Number(right.order) || String(left.id).localeCompare(String(right.id)))
    void (/** @type {any} */ (globalThis)).__tc_rerender?.().then(() => {
      window.dispatchEvent(new CustomEvent('caduceus:external-mailboxes', {
        detail: { accounts: this.$externalAccounts, folders: this.$externalFolders },
      }))
    })
  }

  /**
   * Store display labels locally after loading or saving preferences.
   *
   * @param {Record<string, string> | null | undefined} names
   * @returns {void}
   */
  setMailboxNames(names) {
    const allowed = new Set(this.$domains.map(domain => String(domain).toLowerCase()))
    /** @type {Record<string, string>} */
    const normalized = {}
    for (const [rawDomain, rawName] of Object.entries(names || {})) {
      const domain = rawDomain.toLowerCase()
      const name = typeof rawName === 'string' ? rawName.trim() : ''
      if (allowed.has(domain) && name) normalized[domain] = name
    }
    this.$mailboxNames = normalized
    sessionStorage.setItem('caduceus_mailbox_names', JSON.stringify(normalized))
    window.dispatchEvent(new CustomEvent('caduceus:mailbox-names', { detail: normalized }))
  }

  /**
   * Apply and cache the current user's catalog-backed typography preferences.
   * CSS variables update the shell, DuVay controls, and composer atomically.
   *
   * @param {{ interfaceFont?: unknown, composerFont?: unknown }} preferences
   */
  setFontPreferences(preferences) {
    const normalized = persistFontPreferences({
      interfaceFont: preferences?.interfaceFont ?? this.$interfaceFont,
      composerFont: preferences?.composerFont ?? this.$composerFont,
    })
    this.$interfaceFont = normalized.interfaceFont
    this.$composerFont = normalized.composerFont
    window.dispatchEvent(new CustomEvent('caduceus:font-preferences', { detail: normalized }))
    return normalized
  }

  /** Apply and cache a server-validated interface locale identifier. */
  setInterfaceLocale(locale) {
    const applied = persistInterfaceLocale(locale)
    this.$interfaceLocale = applied.choice
    return applied
  }

  /** @param {unknown} sounds */
  setNotificationSounds(sounds) {
    this.$notificationSounds = persistNotificationSounds(sounds)
    window.dispatchEvent(new CustomEvent('caduceus:notification-sounds', { detail: this.$notificationSounds }))
    return this.$notificationSounds
  }

  /** @param {unknown} preferences */
  setThemePreferences(preferences) {
    const input = preferences && typeof preferences === 'object' && !Array.isArray(preferences)
      ? /** @type {Record<string, unknown>} */ (preferences)
      : {}
    const applied = persistThemePreferences({ ...this.$themePreferences, ...input })
    this.$themePreferences = normalizeThemePreferences(applied)
    this.applyAccent()
    window.dispatchEvent(new CustomEvent('caduceus:theme-preferences', { detail: this.$themePreferences }))
    return this.$themePreferences
  }

  /** Apply the authenticated user's aggregate/focused unread badge scope. */
  setUnreadBadgePreference(preferences) {
    const normalized = normalizeUnreadBadgePreference(preferences, this.$domains)
    window.dispatchEvent(new CustomEvent('caduceus:unread-badge-preference', {
      detail: { ...normalized, domains: [...this.$domains] },
    }))
    return normalized
  }

  /** Apply the authenticated user's validated command overrides. */
  async setKeyboardShortcutOverrides(overrides) {
    const result = await window._caduceusKeyboard?.applyOverrides?.(
      overrides && typeof overrides === 'object' && !Array.isArray(overrides) ? overrides : {},
    )
    return result || { ok: false, error: 'Keyboard command runtime is unavailable.' }
  }

  /** Apply and cache the current user's complete mail-client preference model. */
  setMailPreferences(preferences) {
    this.$mailPreferences = persistMailPreferences(preferences)
    window.dispatchEvent(new CustomEvent('caduceus:mail-preferences', { detail: this.$mailPreferences }))
    return this.$mailPreferences
  }

  /** Persist a complete theme choice without making UI changes depend on the network. */
  async saveThemePreferences() {
    try {
      await this.apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ themePreferences: this.$themePreferences }),
      })
    } catch {
      // The local selection remains usable and is revalidated on the next load.
    }
  }

  /** @param {unknown} event @param {unknown} [soundId] */
  playSound(event, soundId) {
    return playNotificationSound(event, {
      preferences: this.$notificationSounds,
      ...(soundId === undefined ? {} : { soundId }),
    })
  }

  /**
   * Return a personal mailbox label while retaining the domain as its key.
   *
   * @param {string} domain
   * @returns {string}
   */
  mailboxLabel(domain) {
    const normalized = String(domain || '').trim().toLowerCase()
    return this.$mailboxNames[normalized] || normalized
  }

  externalMailboxLabel(account) {
    return String(account?.label || account?.mailboxAddress || 'Connected mailbox')
  }

  /**
   * Registers the global `window._caduceus` API surface once.
   *
   * Exposes reactive auth state, API fetch helpers, toast notifications,
   * navigation, email composition, and formatting utilities so that child
   * components and pages can interact with the app shell without importing it
   * directly.
   */
  registerGlobals() {
    if (window._caduceusInitialised && window._caduceus?.auth?.token === this.$token) return
    window._caduceusInitialised = true

    const self = this
    window._caduceus = {
      get auth() { return { get token() { return self.$token }, get email() { return self.$email }, get name() { return self.$name }, get displayName() { return self.displayName }, get role() { return self.$role }, get domains() { return self.$domains }, get mailboxNames() { return self.$mailboxNames }, get externalAccounts() { return self.$externalAccounts }, get externalFolders() { return self.$externalFolders }, get isLoggedIn() { return !!self.$token } } },
      get accentColor() { return self.$$accentColor },
      get accentPresets() { return ACCENT_PRESETS },
      get interfaceFont() { return self.$interfaceFont },
      get composerFont() { return self.$composerFont },
      get interfaceLocale() { return currentInterfaceLocale() },
      get interfaceLocaleChoice() { return currentInterfaceLocaleChoice() },
      get notificationSounds() { return self.$notificationSounds },
      get themePreferences() { return self.$themePreferences },
      get mailPreferences() { return self.$mailPreferences },
      get screenshotMode() { return isScreenshotModeEnabled() },
      setAccent: (k) => self.setAccent(k),
      setScreenshotMode: (enabled) => self.setScreenshotMode(Boolean(enabled)),
      mailboxLabel: (d) => self.mailboxLabel(d),
      setMailboxNames: (n) => self.setMailboxNames(n),
      setFontPreferences: (preferences) => self.setFontPreferences(preferences),
      setInterfaceLocale: (locale) => self.setInterfaceLocale(locale),
      setNotificationSounds: (sounds) => self.setNotificationSounds(sounds),
      setThemePreferences: (preferences) => self.setThemePreferences(preferences),
      setUnreadBadgePreference: (preferences) => self.setUnreadBadgePreference(preferences),
      setKeyboardShortcutOverrides: (overrides) => self.setKeyboardShortcutOverrides(overrides),
      setMailPreferences: (preferences) => self.setMailPreferences(preferences),
      cachePreferences: (response) => self.cachePreferences(response),
      playSound: (event, soundId) => self.playSound(event, soundId),
      apiFetch: (p, o) => this.apiFetch(p, o),
      apiFetchCached: (p, o) => this.apiFetchCached(p, o),
      toast: (m, d) => this.toast(m, d),
      toastAction: (msg, action, duration) => this.toastAction(msg, action, duration),
      queueOfflineRequest: (request) => this.queueOfflineRequest(request),
      performMailAction: (ids, patch, message) => this.performMailAction(ids, patch, message),
      navigate: (t) => this.navigate(t),
      openEmail: (e) => this.openEmail(e),
      openEmailId: (id) => this.openEmailId(id),
      reportSpam: (id) => this.reportSpam(id),
      markNotSpam: (id) => this.markNotSpam(id),
      compose: (p) => this.compose(p),
      consumeComposePrefill: () => this.consumeComposePrefill(),
      handleLogin: (d, r) => this.handleLogin(d, r),
      initials: (a) => this.initials(a),
      bytesLabel: (s) => this.bytesLabel(s),
      formatDate: (i) => formatLocalizedDate(i),
      translate: (id, values) => translate(id, values),
    }
  }

  /**
   * Toggle the open/closed state of the topbar user-menu dropdown.
   */
  toggleUserMenu() {
    this.mobileSidebarOpen = false
    this.userMenuOpen = !this.userMenuOpen
  }

  /**
   * Force-close the user-menu dropdown (used by menu-item click handlers
   * so the menu dismisses after the user picks something).
   */
  closeUserMenu() { this.userMenuOpen = false }

  /** Toggle the reversible, tab-scoped PII masking used for demonstrations. */
  toggleScreenshotMode() { this.setScreenshotMode(!this.$screenshotMode) }

  /**
   * Apply screenshot privacy without placing the unmasked values in a second
   * DOM tree. The control itself remains readable so the user can always exit.
   *
   * @param {boolean} enabled
   */
  setScreenshotMode(enabled) {
    this.$screenshotMode = setScreenshotMode(enabled)
    this.userMenuOpen = false
    window._caduceus?.toast(enabled ? 'Screenshot mode enabled.' : 'Screenshot mode disabled.')
    setTimeout(() => setScreenshotMode(enabled), 0)
  }

  toggleMobileSidebar() {
    this.userMenuOpen = false
    this.mobileSidebarOpen = !this.mobileSidebarOpen
  }

  closeMobileSidebar() { this.mobileSidebarOpen = false }

  /**
   * Nudge inbox-list components after the shell API has been registered.
   *
   * Direct authenticated routes can mount child components before the shell
   * globals are fully visible. A few delayed pulses keep this cheap and make
   * the inbox resilient to mount-order differences.
   *
   * @returns {void}
   */
  pulseInboxReady() {
    for (const delay of [0, 50, 250, 1000]) {
      setTimeout(() => {
        window.dispatchEvent(new CustomEvent('caduceus:refresh-inbox'))
      }, delay)
    }
  }

  /**
   * Return the count for a folder, or empty string if zero (so the template
   * can render `<span class="folder-count">{folderCount('inbox')}</span>`
   * without showing a "0" badge for empty folders).
   *
   * @param {string} name - Folder name.
   * @returns {number | string}
   */
  folderCount(name) {
    if (!this.$mailPreferences.showUnreadForAllFolders && name !== 'inbox') return ''
    const n = this.folderCounts?.[name]
    return n && n > 0 ? n : ''
  }

  /**
   * Per-domain folder count for the Favorites and per-domain sidebar rows.
   *
   * @param {string} folder
   * @param {string} domain
   * @returns {number | string}
   */
  domainFolderCount(folder, domain) {
    if (!this.$mailPreferences.showUnreadForAllFolders && folder !== 'inbox') return ''
    const n = this.folderCountsByDomain?.[domain]?.[folder]
    return n && n > 0 ? n : ''
  }

  accountFolderCount(folder, accountId) {
    if (!this.$mailPreferences.showUnreadForAllFolders && folder !== 'inbox') return ''
    const n = this.folderCountsByAccount?.[accountId]?.[folder]
    return n && n > 0 ? n : ''
  }

  externalFoldersForAccount(accountId) {
    return visibleExternalFolderRows(this.$externalFolders?.[accountId] || [], this.externalFolderOpen)
  }

  externalFolderActions(folder) {
    const account = this.$externalAccounts.find(candidate => candidate.id === folder?.accountId)
    return externalFolderActions(folder, account)
  }

  hasExternalFolderActions(folder) {
    return Object.values(this.externalFolderActions(folder)).some(Boolean)
  }

  canCreateExternalRootFolder(account) {
    return account?.capabilities?.mailWrite === true && !['paused', 'auth_required', 'deleting'].includes(String(account?.state || ''))
  }

  get externalFolderDialogTitle() {
    if (this.externalFolderDialogMode === 'rename') return 'Rename connected folder'
    if (this.externalFolderDialogMode === 'delete') return 'Delete connected folder'
    return this.externalFolderDialogFolder ? 'Create subfolder' : 'Create connected folder'
  }

  get externalFolderDialogAction() {
    if (this.externalFolderDialogBusy) return 'Working…'
    if (this.externalFolderDialogMode === 'rename') return 'Rename folder'
    if (this.externalFolderDialogMode === 'delete') return 'Delete folder'
    return 'Create folder'
  }

  get externalFolderDialogDescription() {
    const folder = this.externalFolderDialogFolder
    if (this.externalFolderDialogMode === 'delete') return `Delete “${folder?.displayName || folder?.name || 'this folder'}” from the connected mailbox? The provider may also delete messages it contains.`
    if (this.externalFolderDialogMode === 'rename') return `Choose a new name for “${folder?.displayName || folder?.name || 'this folder'}”.`
    return folder ? `Create a folder inside “${folder.displayName || folder.name}”.` : 'Create a folder at the top level of this connected mailbox.'
  }

  get externalFolderDialogAccount() {
    return this.$externalAccounts.find(account => account.id === this._externalFolderDialogAccountId) || null
  }

  get externalFolderDialogFolder() {
    return (this.$externalFolders?.[this._externalFolderDialogAccountId] || [])
      .find(folder => folder.stableId === this._externalFolderDialogFolderId) || null
  }

  /** @param {Event} event @param {'create'|'rename'|'delete'} mode @param {Record<string, any>} account @param {Record<string, any>} [folder] */
  openExternalFolderDialog(event, mode, account, folder = undefined) {
    const resolvedAccount = folder?.accountId
      ? this.$externalAccounts.find(candidate => candidate.id === folder.accountId)
      : account
    if (!resolvedAccount || mode === 'create' && !this.canCreateExternalRootFolder(resolvedAccount) && !folder) return
    const actions = folder ? this.externalFolderActions(folder) : null
    if (mode === 'create' && folder && !actions?.createChild || mode === 'rename' && !actions?.rename || mode === 'delete' && !actions?.delete) return
    const details = event.currentTarget instanceof Element ? event.currentTarget.closest('details') : null
    if (details instanceof HTMLDetailsElement) details.open = false
    this._externalFolderDialogReturnFocus = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
    this._externalFolderDialogAccountId = String(resolvedAccount.id || '')
    this._externalFolderDialogFolderId = String(folder?.stableId || '')
    this.externalFolderDialogMode = mode
    this.externalFolderDialogName = mode === 'rename' ? String(folder?.displayName || folder?.name || '') : ''
    this.externalFolderDialogError = ''
    this.externalFolderDialogOpen = true
    void (/** @type {any} */ (globalThis)).__tc_rerender?.().then(() => {
      const target = document.querySelector(mode === 'delete' ? '[data-external-folder-submit]' : '[data-external-folder-name]')
      if (target instanceof HTMLElement) target.focus()
    })
  }

  closeExternalFolderDialog() {
    if (!this.externalFolderDialogOpen || this.externalFolderDialogBusy) return
    this.externalFolderDialogOpen = false
    const restore = this._externalFolderDialogReturnFocus
    this._externalFolderDialogReturnFocus = null
    this._externalFolderDialogAccountId = ''
    this._externalFolderDialogFolderId = ''
    void (/** @type {any} */ (globalThis)).__tc_rerender?.().then(() => restore?.isConnected && restore.focus())
  }

  externalFolderDialogBackdrop(event) {
    if (event.target === event.currentTarget) this.closeExternalFolderDialog()
  }

  externalFolderDialogKeydown(event) {
    if (event.key === 'Escape') {
      event.preventDefault()
      this.closeExternalFolderDialog()
      return
    }
    if (event.key !== 'Tab') return
    const dialog = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
    if (!dialog) return
    const focusable = [...dialog.querySelectorAll('button, input, [tabindex]:not([tabindex="-1"])')]
      .filter(element => element instanceof HTMLElement && !element.hasAttribute('disabled'))
    if (!focusable.length) { event.preventDefault(); dialog.focus(); return }
    const first = /** @type {HTMLElement} */ (focusable[0])
    const last = /** @type {HTMLElement} */ (focusable[focusable.length - 1])
    if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog)) { event.preventDefault(); last.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
  }

  async submitExternalFolderDialog(event) {
    event.preventDefault()
    if (this.externalFolderDialogBusy) return
    const account = this.externalFolderDialogAccount
    const folder = this.externalFolderDialogFolder
    const mode = this.externalFolderDialogMode
    const name = this.externalFolderDialogName
    if (!account || mode !== 'delete' && (!name.trim() || name !== name.trim() || name.length > 128 || /[\0\r\n]/.test(name))) {
      this.externalFolderDialogError = 'Enter a folder name of 128 characters or fewer without leading or trailing spaces.'
      return
    }
    this.externalFolderDialogBusy = true
    this.externalFolderDialogError = ''
    try {
      const body = {
        revision: account.revision,
        idempotencyKey: `folder-${mode}-${crypto.randomUUID()}`,
        kind: `folder_${mode}`,
        messageStableIds: [],
        folderStableId: mode === 'create' ? '' : String(folder?.stableId || ''),
        parentStableId: mode === 'create' ? String(folder?.stableId || '') : '',
        folderName: mode === 'delete' ? '' : name,
      }
      const response = await this.apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}/mutations`, {
        method: 'POST', body: JSON.stringify(body),
      })
      const result = await response?.json().catch(() => null)
      if (!response?.ok || !result?.id) throw new Error(result?.error || `Could not ${mode} the connected folder.`)
      this.externalFolderDialogBusy = false
      this.closeExternalFolderDialog()
      this.toastAction(`Folder ${mode} queued. Provider folders are syncing.`, {
        label: 'Refresh folders', onClick: () => this.loadExternalMailboxes(),
      })
    } catch (error) {
      this.externalFolderDialogError = error instanceof Error ? error.message : `Could not ${mode} the connected folder.`
    } finally {
      this.externalFolderDialogBusy = false
    }
  }

  /** @param {Record<string, any>} folder */
  isExternalFolderOpen(folder) {
    return this.externalFolderOpen[externalFolderOpenKey(folder.accountId, folder.stableId)] === true
  }

  /** @param {Event} _event @param {Record<string, any>} folder */
  toggleExternalFolder(_event, folder) {
    if (!folder?.hasChildren) return
    const key = externalFolderOpenKey(folder.accountId, folder.stableId)
    this.externalFolderOpen = { ...this.externalFolderOpen, [key]: !this.externalFolderOpen[key] }
  }

  /** @param {Record<string, any>} folder */
  externalFolderRowStyle(folder) {
    const depth = Math.max(0, Math.min(8, Number(folder?.depth) || 0))
    return `--external-folder-depth:${depth}`
  }

  externalFolderHref(folder, accountId) {
    const query = `account=${encodeURIComponent(accountId)}`
    return folder === 'inbox'
      ? `/inbox?${query}`
      : `/folder/${encodeURIComponent(folder)}?${query}`
  }

  /**
   * Per-domain custom folders (any folder for a domain that isn't a system one).
   *
   * @param {string} domain
   * @returns {string[]}
   */
  customFoldersForDomain(domain) {
    const system = new Set(['inbox', 'drafts', 'sent', 'scheduled', 'reminders', 'snoozed', 'archive', 'spam', 'trash', 'activity'])
    return Object.keys(this.folderCountsByDomain?.[domain] || {})
      .filter(name => name && !system.has(name))
      .sort()
  }

  /**
   * Custom folders discovered from loaded mailbox counts. Rule-created
   * folders begin appearing here as soon as messages are moved into them.
   *
   * @returns {string[]}
   */
  get customFolders() {
    const system = new Set(['inbox', 'drafts', 'sent', 'scheduled', 'reminders', 'snoozed', 'archive', 'spam', 'trash', 'calendar', 'team', 'compose', 'settings', 'activity'])
    return Object.keys(this.folderCounts || {}).filter(name => name && !system.has(name)).sort()
  }

  /**
   * @param {string} name
   * @returns {string}
   */
  folderLabel(name) {
    return String(name || 'folder')
      .split('/')
      .filter(Boolean)
      .map(part => part.charAt(0).toUpperCase() + part.slice(1))
      .join(' / ')
  }

  /**
   * Apply the accent-color preset to the document by overriding the
   * `--ms-primary`/`--ms-primary-container`/`--ms-on-primary-container`
   * custom properties on `<html>`. Falls back to the default Mailspring teal
   * when the saved key isn't recognised.
   *
   * Runs on init and after the user selects a different preset via
   * Settings → Appearance.
   */
  applyAccent() {
    const preset = ACCENT_PRESETS[this.$$accentColor] || ACCENT_PRESETS.teal
    const root = document.documentElement
    root.style.setProperty('--ms-primary', preset.primary)
    root.style.setProperty('--ms-primary-container', preset.container)
    root.style.setProperty('--ms-on-primary-container', preset.onContainer)
    root.dataset.accent = this.$$accentColor
  }

  /**
   * Switch the accent color and re-apply it. Exposed on `window._caduceus`
   * so the Settings component can call it without importing the page class.
   *
   * @param {string} key - One of the keys in {@link ACCENT_PRESETS}.
   */
  setAccent(key) {
    if (!ACCENT_PRESETS[key]) return
    this.$$accentColor = key
    this.applyAccent()
  }

  /**
   * Toggles the colour theme between light and dark.
   *
   * Updates the `w-theme` (DuVay tokens) and `data-theme` (toggle-icon CSS)
   * attributes on `<html>` and persists the choice to localStorage so it
   * survives page reloads.
   */
  toggleTheme() {
    const resolved = resolveTheme(this.$themePreferences)
    const next = themeBaseMode(resolved) === 'dark' ? 'light' : 'dark'
    this.setThemePreferences({ ...this.$themePreferences, activeTheme: next })
    void this.saveThemePreferences()
  }

  get themeToggleLabel() {
    const resolved = resolveTheme(this.$themePreferences)
    return themeBaseMode(resolved) === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'
  }

  /**
   * Toggle a sidebar disclosure without reconciling the routed page slot.
   * The section owns this visual state; re-rendering the shell here can
   * briefly replace or resize the mail list and message-detail columns.
   *
   * @param {Event} event
   * @param {string} name
   */
  toggleSection(event, name) {
    const eventTarget = event?.target instanceof Element ? event.target : null
    const currentTarget = event?.currentTarget instanceof Element ? event.currentTarget : null
    const trigger = /** @type {HTMLElement | null} */ (
      eventTarget?.closest(`[data-sidebar-section-toggle="${name}"]`)
      || currentTarget?.closest(`[data-sidebar-section-toggle="${name}"]`)
      || document.querySelector(`[data-sidebar-section-toggle="${name}"]`)
    )
    if (!trigger) return
    const section = trigger.closest('.sidebar-section') || document.querySelector(`[data-sidebar-section="${name}"]`)
    if (!section) return

    const collapsed = !section.classList.contains('collapsed')
    section.classList.toggle('collapsed', collapsed)
    trigger.setAttribute('aria-expanded', String(!collapsed))

    try {
      localStorage.setItem(`caduceus-sidebar-section-${name}`, collapsed ? 'closed' : 'open')
    } catch {
      // Persistence is optional when storage is unavailable.
    }
  }

  /**
   * Read the persisted visual state for a sidebar disclosure.
   * @param {string} name
   * @returns {boolean}
   */
  sectionExpanded(name) {
    try {
      const stored = localStorage.getItem(`caduceus-sidebar-section-${name}`)
      if (stored === 'open' || stored === 'closed') return stored === 'open'
    } catch {
      // Fall back to the initial shell state during server rendering.
    }

    return this.sidebarOpen?.[name] !== false
  }

  /**
   * Helper for templates: returns the section's CSS class with `collapsed`
   * appended when the section is closed.
   * @param {string} name
   * @returns {string}
   */
  sectionClass(name) {
    return this.sectionExpanded(name) ? 'sidebar-section' : 'sidebar-section collapsed'
  }

  /**
   * Open/close one of the collapsible Favorites groups ("inboxes" or "sent").
   *
   * @param {Event} _event
   * @param {'inboxes' | 'sent'} name
   */
  toggleFavorite(_event, name) {
    const expanded = this.favoritesOpen?.[name] !== false
    this.favoritesOpen = { ...this.favoritesOpen, [name]: !expanded }
    const group = document.querySelector(`[data-favorite-group="${name}"]`)
    const trigger = document.querySelector(`[data-favorite-group-toggle="${name}"]`)
    group?.classList.toggle('collapsed', expanded)
    trigger?.setAttribute('aria-expanded', String(!expanded))
  }

  /**
   * CSS class for a collapsible Favorites group. Adds `collapsed` when the
   * group is closed so the chevron rotates and the children hide.
   *
   * @param {'inboxes' | 'sent'} name
   * @returns {string}
   */
  favoriteGroupClass(name) {
    return this.favoritesOpen?.[name] === false ? 'favorite-group collapsed' : 'favorite-group'
  }

  /**
   * Determines the currently active folder from the URL path and sets
   * `$activeFolder` so the sidebar can highlight the matching nav item.
   */
  updateActiveFolder() {
    const parts = location.pathname.split('/')
    const segment = parts[1]
    // /folder/<name> → active folder is <name>
    if (segment === 'folder') { this.activeFolder = parts[2] || 'inbox' }
    else if (segment === 'email' || location.pathname === '/') { this.activeFolder = 'inbox' }
    // The inbox route also hosts the Snoozed folder via `/inbox?folder=snoozed`
    // (every other folder uses `/folder/<name>`), so read the query param here —
    // otherwise the sidebar would keep Inbox highlighted for Snoozed.
    else if (segment === 'inbox') {
      this.activeFolder = new URLSearchParams(location.search).get('folder') || 'inbox'
    }
    else {
      const FOLDERS = new Set(['inbox', 'settings', 'compose', 'scheduled', 'reminders', 'snoozed', 'contacts', 'calendar', 'team', 'activity'])
      if (FOLDERS.has(segment)) this.activeFolder = segment
    }
    // Track per-domain selection from `?domain=X` so the sidebar can
    // distinguish "All Inboxes" from "Inbox / caduceus.com" when both
    // share the same activeFolder value.
    try {
      const params = new URLSearchParams(location.search)
      this.selectedDomain = (params.get('domain') || '').toLowerCase()
      this.selectedAccount = params.get('account') || ''
    } catch {
      this.selectedDomain = ''
      this.selectedAccount = ''
    }
  }

  /**
   * Binds global event listeners for logout, email opening, navigation, hash
   * changes, sign-out clicks, and keyboard shortcuts.
   */
  bindEvents() {
    window.addEventListener('caduceus:logout', () => this.signOut())
    window.addEventListener('caduceus:pending-send-undo', () => this.restorePendingSendUndo())
    window.addEventListener('caduceus:open-email', (e) => { this.openEmail(e.detail) })
    window.addEventListener('tachyon:navigate', () => {
      this.mobileSidebarOpen = false
      this.updateActiveFolder()
      if (location.pathname !== '/activity') void this.refreshActivityCount()
      setTimeout(() => this.bootstrapActivePage(), 0)
      setTimeout(() => this.restorePendingSendUndo(), 0)
    })
    window.addEventListener('caduceus:activity-viewed', () => {
      this.folderCounts = { ...this.folderCounts, activity: 0 }
    })
    window.addEventListener('hashchange', () => {
      if (!location.hash.startsWith('#email=')) return
      this.openEmailId(decodeURIComponent(location.hash.slice('#email='.length)))
    })
    document.addEventListener('click', (e) => {
      if (!/** @type {Element} */ (e.target)?.closest?.('[data-sign-out]')) return
      e.preventDefault()
      this.signOut()
    }, true)

    // Disclosure controls are intentionally delegated outside Tachyon's
    // template event pipeline. Their state is local to the sidebar and must
    // not trigger a document-wide render that can disturb the mail columns.
    document.addEventListener('click', (event) => {
      const target = /** @type {Element | null} */ (event.target instanceof Element ? event.target : null)
      const sectionToggle = target?.closest?.('[data-sidebar-section-toggle]')
      if (sectionToggle) {
        this.toggleSection(event, sectionToggle.getAttribute('data-sidebar-section-toggle') || '')
        return
      }
      const favoriteToggle = target?.closest?.('[data-favorite-group-toggle]')
      const name = favoriteToggle?.getAttribute('data-favorite-group-toggle')
      if (name === 'inboxes' || name === 'sent') this.toggleFavorite(event, name)
    }, true)

    // Close the user-menu dropdown when the user clicks anywhere outside it.
    // Capture phase + simple closest() check is enough — items inside the
    // dropdown call closeUserMenu() explicitly so this handler stays cheap.
    document.addEventListener('click', (e) => {
      if (!this.userMenuOpen) return
      const target = /** @type {Element} */ (e.target)
      if (target?.closest?.('.app-user-menu')) return
      this.userMenuOpen = false
    })

    document.addEventListener('keydown', (e) => {
      if (e.key !== 'Escape' || !this.mobileSidebarOpen) return
      this.mobileSidebarOpen = false
    })

    document.addEventListener('keydown', (event) => {
      if (!this.shortcutHelpOpen) return
      if (event.key === 'Escape') {
        event.preventDefault()
        this.closeShortcutHelp()
        return
      }
      if (event.key !== 'Tab') return
      const dialog = document.querySelector('[data-shortcut-help-dialog]')
      if (!(dialog instanceof HTMLElement)) return
      const focusable = [...dialog.querySelectorAll('button, a[href], [tabindex]:not([tabindex="-1"])')]
        .filter(element => element instanceof HTMLElement && !element.hasAttribute('disabled'))
      if (!focusable.length) { event.preventDefault(); dialog.focus(); return }
      const first = /** @type {HTMLElement} */ (focusable[0])
      const last = /** @type {HTMLElement} */ (focusable[focusable.length - 1])
      if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog)) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && (document.activeElement === last || document.activeElement === dialog)) { event.preventDefault(); first.focus() }
    })

    // Per-folder counts published by inbox-view and refreshFolderCounts.
    // Supports both the legacy flat shape (`Record<folder, n>`) and the new
    // domain-aware shape (`{ counts, byDomain }`) so older publishers keep
    // working while the per-domain sidebar reads `folderCountsByDomain`.
    window.addEventListener('caduceus:folder-counts', (e) => {
      const detail = /** @type {CustomEvent<any>} */ (e).detail
      if (!detail || typeof detail !== 'object') return
      if ('counts' in detail || 'byDomain' in detail || 'byAccount' in detail) {
        if (detail.counts && typeof detail.counts === 'object') this.folderCounts = { ...detail.counts, activity: this.folderCounts.activity || 0 }
        if (detail.byDomain && typeof detail.byDomain === 'object') this.folderCountsByDomain = { ...detail.byDomain }
        if (detail.byAccount && typeof detail.byAccount === 'object') this.folderCountsByAccount = { ...detail.byAccount }
      } else {
        this.folderCounts = { ...detail, activity: this.folderCounts.activity || 0 }
      }
    })

    // ── Keyboard shortcut event listeners ──
    this._bindShortcut('core:compose', () => this.compose({}))
    this._bindShortcut('core:go-inbox', () => this.navigate('/inbox'))
    this._bindShortcut('core:go-drafts', () => this.navigate('/drafts'))
    this._bindShortcut('core:go-sent', () => this.navigate('/sent'))
    this._bindShortcut('core:go-archive', () => this.navigate('/archive'))
    this._bindShortcut('core:go-spam', () => this.navigate('/spam'))
    this._bindShortcut('core:go-trash', () => this.navigate('/trash'))
    this._bindShortcut('core:go-settings', () => this.navigate('/settings'))
    this._bindShortcut('core:toggle-screenshot-mode', () => this.toggleScreenshotMode())
    this._bindShortcut('email:report-spam', () => { /* contextual inbox/detail components handle the selected message */ })
    this._bindShortcut('core:refresh', () => {
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
    })
    this._bindShortcut('core:search', () => {
      const field = document.querySelector('[data-thread-search-input]')
      if (field) /** @type {HTMLElement} */ (field).focus()
    })
    this._bindShortcut('core:show-shortcuts', () => this.openShortcutHelp())
  }

  get shortcutHelpGroups() { return window._caduceusKeyboard?.getGroups?.() || [] }

  openShortcutHelp() {
    if (this.shortcutHelpOpen) return
    const active = document.activeElement instanceof HTMLElement ? document.activeElement : null
    this._shortcutHelpReturnFocus = active
    this._shortcutHelpReturnIdentity = active ? {
      id: active.id,
      tagName: active.tagName.toLowerCase(),
      ariaLabel: active.getAttribute('aria-label') || '',
      href: active.getAttribute('href') || '',
      name: active.getAttribute('name') || '',
      type: active.getAttribute('type') || '',
    } : null
    this.shortcutHelpOpen = true
    void (/** @type {any} */ (globalThis)).__tc_rerender?.().then(() => {
      const dialog = document.querySelector('[data-shortcut-help-dialog]')
      if (dialog instanceof HTMLElement) dialog.focus()
    })
  }

  closeShortcutHelp() {
    if (!this.shortcutHelpOpen) return
    this.shortcutHelpOpen = false
    const restore = this._shortcutHelpReturnFocus
    const identity = this._shortcutHelpReturnIdentity
    this._shortcutHelpReturnFocus = null
    this._shortcutHelpReturnIdentity = null
    void (/** @type {any} */ (globalThis)).__tc_rerender?.().then(() => {
      if (restore?.isConnected) { restore.focus(); return }
      if (!identity) return
      if (identity.id) {
        const byId = document.getElementById(identity.id)
        if (byId instanceof HTMLElement) { byId.focus(); return }
      }
      const candidates = [...document.querySelectorAll(identity.tagName)]
      const replacement = candidates.find(candidate =>
        (!identity.ariaLabel || candidate.getAttribute('aria-label') === identity.ariaLabel) &&
        (!identity.href || candidate.getAttribute('href') === identity.href) &&
        (!identity.name || candidate.getAttribute('name') === identity.name) &&
        (!identity.type || candidate.getAttribute('type') === identity.type)
      )
      if (replacement instanceof HTMLElement) replacement.focus()
    })
  }

  shortcutHelpBackdrop(event) {
    if (event.target === event.currentTarget) this.closeShortcutHelp()
  }

  openShortcutSettings() {
    this.shortcutHelpOpen = false
    this.navigate('/settings#keyboard-shortcuts')
  }

  /**
   * Bridge service-worker notification buttons into the authenticated mailbox
   * action boundary. The worker never receives or stores a bearer token: it
   * sends an allow-listed intent to an open client, or opens a same-origin URL
   * that is consumed only after this authenticated shell has mounted.
   */
  bindNotificationActions() {
    if (!this.isAuthenticated) return
    if ('serviceWorker' in navigator && !window._caduceusNotificationAction) {
      window._caduceusNotificationAction = (event) => {
        if (event.data?.type !== 'caduceus:notification-action') return
        void this.runNotificationAction(event.data.action, event.data.emailId)
      }
      navigator.serviceWorker.addEventListener('message', window._caduceusNotificationAction)
    }

    const params = new URLSearchParams(location.search)
    const action = params.get('notificationAction') || ''
    const emailId = params.get('notificationEmail') || ''
    if (!action && !emailId) return
    params.delete('notificationAction')
    params.delete('notificationEmail')
    const clean = `${location.pathname}${params.size ? `?${params}` : ''}${location.hash}`
    history.replaceState(history.state, '', clean)
    void this.runNotificationAction(action, emailId)
  }

  /** @param {unknown} action @param {unknown} emailId */
  async runNotificationAction(action, emailId) {
    if (typeof emailId !== 'string' || !/^[A-Za-z0-9._:-]{1,200}$/.test(emailId)) return
    if (action === 'mark-read') {
      await this.performMailAction([emailId], { read: true }, 'Marked read from notification.')
    } else if (action === 'archive') {
      await this.performMailAction([emailId], { folder: 'archive' }, 'Archived from notification.')
    }
  }

  /**
   * Consume files delivered by the installed application's manifest file
   * handler. EML content crosses the same authenticated, size-bounded import
   * route as the visible Inbox picker and defaults to the signed-in primary
   * mailbox's Inbox.
   */
  bindPwaLaunchQueue() {
    const launchQueue = /** @type {any} */ (window).launchQueue
    if (!this.isAuthenticated || !launchQueue?.setConsumer || window._caduceusPwaLaunchBound) return
    window._caduceusPwaLaunchBound = true
    launchQueue.setConsumer(async (launchParams) => {
      const handles = Array.isArray(launchParams?.files) ? launchParams.files.slice(0, 20) : []
      if (!handles.length) return
      let imported = 0
      for (const handle of handles) {
        try {
          const file = await handle.getFile()
          if (!file || file.size === 0 || file.size > 50 * 1024 * 1024 || !/\.eml$/i.test(file.name)) continue
          const contentBase64 = await this.fileBase64(file)
          const response = await this.apiFetch('/inbox/import/eml', {
            method: 'POST',
            body: JSON.stringify({ mailbox: this.$email, folder: 'inbox', contentBase64 }),
          })
          if (response?.ok) imported += 1
        } catch {}
      }
      if (imported) {
        this.toast(`Imported ${imported} ${imported === 1 ? 'message' : 'messages'} from the file handler.`)
        this.navigate('/inbox')
        window.dispatchEvent(new Event('caduceus:refresh-inbox'))
      } else {
        this.toast('No supported EML file could be imported.')
      }
    })
  }

  /** @param {File} file */
  async fileBase64(file) {
    return await new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onerror = () => reject(reader.error || new Error('Could not read EML file'))
      reader.onload = () => {
        const result = String(reader.result || '')
        const comma = result.indexOf(',')
        if (comma < 0) reject(new Error('Could not encode EML file'))
        else resolve(result.slice(comma + 1))
      }
      reader.readAsDataURL(file)
    })
  }

  /** Reconcile queued mailbox writes after reconnect or a Background Sync wake. */
  bindOfflineMailActions() {
    if (!this.isAuthenticated || window._caduceusOfflineActionsBound) return
    window._caduceusOfflineActionsBound = true
    if (this.$email) void migrateLegacyOfflineMailActions(this.$email)
      .then(count => { if (count && navigator.onLine) void this.flushOfflineMailActions(true) })
    window.addEventListener('online', () => { void this.flushOfflineMailActions(true) })
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.addEventListener('message', event => {
        if (event.data?.type === 'caduceus:flush-offline-writes') void this.flushOfflineMailActions(true)
      })
    }
    if (navigator.onLine) void this.flushOfflineMailActions()
  }

  /** @param {boolean} [force] */
  async flushOfflineMailActions(force = false) {
    if (!this.$email || (!force && !navigator.onLine)) return 0
    const summary = await flushOfflineWrites(this.$email, async (item) => {
      const response = await this.apiFetch(item.path, {
        method: item.method,
        ...(item.body ? { body: JSON.stringify(item.body) } : {}),
      })
      if (!response) return { status: 'retry' }
      const data = await response.clone().json().catch(() => null)
      if (response.ok) return { status: 'accepted', response: data }
      if (response.status === 409) return { status: 'conflict', response: data }
      if ([400, 403, 404, 410, 422].includes(response.status)) return { status: 'permanent', response: data }
      return { status: 'retry', response: data }
    })
    for (const result of summary.results) {
      if (result.status === 'accepted' && result.kind === 'mail-action' && result.response?.actionId) {
        this._publishMailAction(result.response)
      }
      window.dispatchEvent(new CustomEvent('caduceus:offline-write-result', { detail: result }))
    }
    if (summary.completed) {
      this.toast(`${summary.completed} queued ${summary.completed === 1 ? 'change' : 'changes'} synchronized.`)
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
    } else if (summary.conflicts) {
      this.toast('A queued change conflicts with newer server data. Reopen it to resolve the conflict.', 7000)
    } else if (summary.failed) {
      this.toast(`${summary.failed} queued ${summary.failed === 1 ? 'change could' : 'changes could'} not be applied.`, 7000)
    }
    return summary.completed
  }

  /**
   * Encrypt and persist an authenticated write without placing its bearer
   * token or plaintext body in durable browser storage.
   * @param {Omit<import('../../../shared/scripts/offline-write-vault.js').OfflineWriteInput, 'owner'> & {owner?: string}} request
   */
  async queueOfflineRequest(request) {
    if (!this.$email) throw new Error('An authenticated mailbox is required')
    const queued = await queueOfflineWrite({ ...request, owner: this.$email })
    this.toast('Change queued securely and will synchronize when this device reconnects.')
    return queued
  }

  /** Run a page's authenticated data bootstrap through Tachyon's event pipeline. */
  bootstrapActivePage() {
    /** @type {HTMLButtonElement | null} */
    const trigger = document.querySelector('[data-caduceus-page-bootstrap]')
    trigger?.click()
  }

  /** Restore persisted desktop column widths and clamp them to this viewport. */
  restoreColumnWidths() {
    if (!matchMedia('(min-width: 1025px)').matches) return
    for (const kind of /** @type {const} */ (['sidebar', 'mail'])) {
      const stored = Number.parseFloat(localStorage.getItem(`caduceus-column-${kind}-width`) || '')
      if (Number.isFinite(stored)) this.setColumnWidth(kind, stored, false)
    }
    requestAnimationFrame(() => {
      for (const kind of /** @type {const} */ (['sidebar', 'mail'])) {
        const property = kind === 'sidebar' ? '--app-sidebar-width' : '--mail-list-width'
        const current = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue(property))
        if (Number.isFinite(current)) this.setColumnWidth(kind, current, false)
      }
    })
  }

  /**
   * Bind delegated pointer and keyboard controls for desktop column handles.
   * Delegation keeps the mail-list handle working across client-side routes.
   */
  bindColumnResizers() {
    if (window._caduceusColumnResizersBound) return
    window._caduceusColumnResizersBound = true

    document.addEventListener('pointerdown', (event) => {
      const pointerEvent = /** @type {PointerEvent} */ (event)
      const handle = /** @type {HTMLElement|null} */ (/** @type {Element} */ (pointerEvent.target)?.closest?.('[data-column-resizer]'))
      if (!handle || pointerEvent.button !== 0 || !matchMedia('(min-width: 1025px)').matches) return
      const kind = handle.dataset.columnResizer
      if (kind !== 'sidebar' && kind !== 'mail') return

      pointerEvent.preventDefault()
      const panel = kind === 'sidebar'
        ? document.querySelector('.app-sidebar')
        : handle.closest('.mail-layout')?.querySelector('.mail-list-panel')
      const startWidth = panel?.getBoundingClientRect().width
      if (!startWidth) return
      const startX = pointerEvent.clientX
      const inlineDirection = getComputedStyle(document.documentElement).direction === 'rtl' ? -1 : 1
      handle.classList.add('is-active')
      document.body.classList.add('column-resize-active')

      const move = (moveEvent) => {
        this.setColumnWidth(kind, startWidth + ((moveEvent.clientX - startX) * inlineDirection), false, handle)
      }
      const stop = () => {
        window.removeEventListener('pointermove', move)
        window.removeEventListener('pointerup', stop)
        window.removeEventListener('pointercancel', stop)
        handle.classList.remove('is-active')
        document.body.classList.remove('column-resize-active')
        const property = kind === 'sidebar' ? '--app-sidebar-width' : '--mail-list-width'
        const width = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue(property))
        if (Number.isFinite(width)) this.setColumnWidth(kind, width, true, handle)
      }
      window.addEventListener('pointermove', move)
      window.addEventListener('pointerup', stop)
      window.addEventListener('pointercancel', stop)
    })

    document.addEventListener('keydown', (event) => {
      const handle = /** @type {HTMLElement|null} */ (/** @type {Element} */ (event.target)?.closest?.('[data-column-resizer]'))
      if (!handle || !['ArrowLeft', 'ArrowRight', 'Home'].includes(event.key)) return
      const kind = handle.dataset.columnResizer
      if (kind !== 'sidebar' && kind !== 'mail') return
      event.preventDefault()
      const property = kind === 'sidebar' ? '--app-sidebar-width' : '--mail-list-width'
      const current = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue(property))
      const inlineDirection = getComputedStyle(document.documentElement).direction === 'rtl' ? -1 : 1
      const next = event.key === 'Home'
        ? (kind === 'sidebar' ? 232 : 390)
        : current + (event.key === 'ArrowLeft' ? -16 : 16) * inlineDirection
      this.setColumnWidth(kind, next, true, handle)
    })

    document.addEventListener('dblclick', (event) => {
      const handle = /** @type {HTMLElement|null} */ (/** @type {Element} */ (event.target)?.closest?.('[data-column-resizer]'))
      const kind = handle?.dataset.columnResizer
      if (!handle || (kind !== 'sidebar' && kind !== 'mail')) return
      this.setColumnWidth(kind, kind === 'sidebar' ? 232 : 390, true, handle)
    })
  }

  /**
   * Apply a clamped desktop column width and update separator accessibility.
   * @param {'sidebar'|'mail'} kind
   * @param {number} requestedWidth
   * @param {boolean} persist
   * @param {HTMLElement|null} [activeHandle]
   */
  setColumnWidth(kind, requestedWidth, persist, activeHandle = null) {
    const mailLayout = activeHandle?.closest('.mail-layout') || document.querySelector('.mail-layout')
    const available = kind === 'sidebar' ? window.innerWidth : mailLayout?.getBoundingClientRect().width || window.innerWidth
    const min = kind === 'sidebar' ? 180 : 280
    const max = kind === 'sidebar'
      ? Math.max(min, Math.min(360, window.innerWidth - 720))
      : Math.max(min, Math.min(640, available - 360))
    const width = Math.round(Math.min(max, Math.max(min, requestedWidth)))
    const property = kind === 'sidebar' ? '--app-sidebar-width' : '--mail-list-width'
    document.documentElement.style.setProperty(property, `${width}px`)
    for (const handle of document.querySelectorAll(`[data-column-resizer="${kind}"]`)) {
      handle.setAttribute('aria-valuemin', String(min))
      handle.setAttribute('aria-valuemax', String(Math.round(max)))
      handle.setAttribute('aria-valuenow', String(width))
    }
    if (persist) localStorage.setItem(`caduceus-column-${kind}-width`, String(width))
  }

  /**
   * Registers a handler for a keyboard-shortcut event.
   *
   * @param {string} name - The shortcut event name (e.g. 'core:compose').
   * @param {() => void} handler - The callback to invoke when the shortcut fires.
   */
  _bindShortcut(name, handler) {
    window.addEventListener('caduceus:shortcut:' + name, handler)
  }

  /**
   * Shows a toast notification for a limited duration.
   *
   * @param {string} msg - The message text to display.
   * @param {number} [duration=2500] - How long the toast stays visible (ms).
   */
  toast(msg, duration = 2500) {
    if (this._toastTimer !== null) clearTimeout(this._toastTimer)
    this.toastMsg = msg
    this.toastActionLabel = ''
    this._toastAction = null
    this.toastVisible = true
    this._toastTimer = setTimeout(() => this.dismissToast(), duration)
  }

  /**
   * Shows a toast notification with an action button.
   *
   * @param {string} msg - The message text to display.
   * @param {{ label: string, onClick: () => unknown }} action - Action descriptor.
   * @param {number} [duration=10000] - How long the toast stays visible (ms).
   */
  toastAction(msg, action, duration = 10000) {
    if (this._toastTimer !== null) clearTimeout(this._toastTimer)
    this.toastMsg = msg
    this.toastActionLabel = action.label
    this._toastAction = action.onClick
    this.toastVisible = true
    this._toastTimer = setTimeout(() => this.dismissToast(), duration)
  }

  /** Hide the shell-owned toast and discard any pending action. */
  dismissToast() {
    if (this._toastTimer !== null) clearTimeout(this._toastTimer)
    this._toastTimer = null
    this.toastVisible = false
    this.toastActionLabel = ''
    this._toastAction = null
  }

  /** Execute the current toast action once. */
  async runToastAction() {
    const action = this._toastAction
    this.dismissToast()
    if (action) await action()
  }

  /**
   * Apply one exact-owner mailbox action and expose its durable undo token.
   * @param {string[]} ids
   * @param {Record<string, any>} patch
   * @param {string} message
   */
  async performMailAction(ids, patch, message) {
    const clientRequestId = crypto.randomUUID()
    let result
    try {
      result = await this._requestMailAction('/inbox/actions', { ids, patch, clientRequestId }, true)
    } catch {
      if (!this.$email) return null
      await this.queueOfflineRequest({
        id: clientRequestId, kind: 'mail-action', owner: this.$email,
        path: '/inbox/actions', method: 'POST', body: { ids, patch, clientRequestId }, message,
      })
      return null
    }
    if (!result) return null
    this._publishMailAction(result)
    this._showMailActionUndo(result, message)
    return result
  }

  /** @param {string} actionId */
  async undoMailAction(actionId) {
    const result = await this._requestMailAction(`/inbox/actions/${encodeURIComponent(actionId)}/undo`)
    if (!result) return null
    this._publishMailAction(result)
    this.toastAction('Action undone.', {
      label: 'Redo',
      onClick: () => this.redoMailAction(result.actionId),
    }, this._mailActionToastDuration(result.expiresAt))
    return result
  }

  /** @param {string} actionId */
  async redoMailAction(actionId) {
    const result = await this._requestMailAction(`/inbox/actions/${encodeURIComponent(actionId)}/redo`)
    if (!result) return null
    this._publishMailAction(result)
    this._showMailActionUndo(result, 'Action redone.')
    return result
  }

  async _requestMailAction(path, body, throwNetworkError = false) {
    try {
      const response = await this.apiFetch(path, {
        method: 'POST',
        ...(body ? { body: JSON.stringify(body) } : {}),
      })
      const result = await response?.json().catch(() => null)
      if (!response?.ok || !result?.actionId) {
        if (response?.status === 409 || result?.conflict) {
          this.toastAction(result?.error || 'Messages changed after this action.', {
            label: 'Refresh',
            onClick: () => this.refreshAfterMailActionConflict(),
          })
        } else {
          this.toast(result?.error || 'Mail action failed.')
        }
        return null
      }
      return result
    } catch (error) {
      if (throwNetworkError) throw error
      this.toast('Mail action failed.')
      return null
    }
  }

  /** Reload authoritative hosted and provider projections after a conflict. */
  refreshAfterMailActionConflict() {
    window.dispatchEvent(new Event('caduceus:refresh-inbox'))
  }

  _publishMailAction(result, refresh = true) {
    window.dispatchEvent(new CustomEvent('caduceus:mail-action', { detail: result }))
    if (refresh) window.dispatchEvent(new Event('caduceus:refresh-inbox'))
  }

  _showMailActionUndo(result, message) {
    const duration = this._mailActionToastDuration(result.expiresAt)
    if (duration <= 0) { this.toast(message); return }
    this.toastAction(message, {
      label: 'Undo',
      onClick: () => this.undoMailAction(result.actionId),
    }, duration)
  }

  _mailActionToastDuration(expiresAt) {
    const remaining = Date.parse(String(expiresAt || '')) - Date.now()
    return Number.isFinite(remaining) ? Math.max(0, Math.min(10_000, remaining)) : 10_000
  }

  /**
   * Programmatically navigates to another client-side route.
   *
   * @param {string} [target='/inbox'] - The destination path (with or without
   *   leading slash).
   */
  navigate(target = '/inbox') {
    const link = document.createElement('a')
    link.href = target.startsWith('/') ? target : `/${target}`
    link.hidden = true
    document.body.appendChild(link)
    link.click()
    link.remove()
  }

  /**
   * Adds a visual drag-over indicator when a draggable item enters a folder.
   *
   * @param {DragEvent} event - The native drag-over event.
   */
  onFolderDragOver(event) {
    event.preventDefault()
    const el = /** @type {HTMLElement} */ (event.currentTarget);
    el.classList.add('drag-over')
  }

  /**
   * Removes the visual drag-over indicator when a draggable item leaves a folder.
   *
   * @param {DragEvent} event - The native drag-leave event.
   */
  onFolderDragLeave(event) {
    const el = /** @type {HTMLElement} */ (event.currentTarget);
    el.classList.remove('drag-over')
  }

  /**
   * Handles dropping an email onto a sidebar folder to move it.
   *
   * Reads the email ID from the drag data, sends a PUT request to the API to
   * update the folder, and triggers an inbox refresh on success.
   *
   * @async
   * @param {string} folder - The target folder name.
   * @param {DragEvent} event - The native drop event.
   * @param {string} [domain] - Optional target domain (per-domain sidebar drops).
   * @returns {Promise<void>}
   */
  async onFolderDrop(folder, event, domain) {
    event.preventDefault()
    const el = /** @type {HTMLElement} */ (event.currentTarget);
    el.classList.remove('drag-over')
    const encodedIds = event.dataTransfer?.getData('application/x-caduceus-mail-ids')
    let ids = []
    try { if (encodedIds) ids = JSON.parse(encodedIds) } catch {}
    if (!Array.isArray(ids) || ids.length === 0) {
      const emailId = event.dataTransfer?.getData('text/plain')
      ids = emailId ? [emailId] : []
    }
    ids = [...new Set(ids.filter(id => typeof id === 'string' && id))]
    if (ids.length === 0 || !folder) return
    const where = domain ? `${folder} (${domain})` : folder
    await this.performMailAction(ids, { folder }, `Moved ${ids.length === 1 ? 'message' : `${ids.length} messages`} to ${where}.`)
  }

  /**
   * Navigates to the email detail page for a given email.
   *
   * @param {string|{ id: string }} email - The email ID string or an object
   *   with an `id` property.
   */
  openEmail(email) {
    const id = typeof email === 'string' ? email : email?.id
    if (id) this.navigate(`/email/${encodeURIComponent(id)}`)
  }

  /**
   * Navigates to the email detail page by raw ID.
   *
   * @param {string} id - The email ID.
   */
  openEmailId(id) { this.openEmail(id) }

  /**
   * Report a message as spam from global callers that do not own an inbox or
   * detail component instance.
   *
   * @async
   * @param {string} id
   * @returns {Promise<void>}
   */
  async reportSpam(id) {
    if (!id) return
    try {
      const res = await this.apiFetch(`/inbox/${id}/spam`, { method: 'PUT', body: JSON.stringify({ spam: true }) })
      if (!res?.ok) { this.toast('Report spam failed.'); return }
      this.toast('Reported spam.')
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
    } catch {
      this.toast('Report spam failed.')
    }
  }

  /**
   * Mark a message as not spam from global callers that do not own an inbox or
   * detail component instance.
   *
   * @async
   * @param {string} id
   * @returns {Promise<void>}
   */
  async markNotSpam(id) {
    if (!id) return
    try {
      const res = await this.apiFetch(`/inbox/${id}/spam`, { method: 'PUT', body: JSON.stringify({ spam: false }) })
      if (!res?.ok) { this.toast('Not spam failed.'); return }
      this.toast('Moved to inbox.')
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
    } catch {
      this.toast('Not spam failed.')
    }
  }

  /**
   * Extracts initials from an email address (up to 2 characters, uppercase).
   *
   * @param {string} addr - An email address string.
   * @returns {string} The uppercase initials derived from the local part.
   */
  initials(addr) { return (addr.split('@')[0] || '?').slice(0, 2).toUpperCase() }

  /** Expose URL encoding to Tachyon's bounded template expression scope. */
  encodeURIComponent(value) { return encodeURIComponent(String(value ?? '')) }

  /**
   * Formats a byte count into a human-readable label (B / KB / MB).
   *
   * @param {number} size - The size in bytes.
   * @returns {string} A human-readable size string.
   */
  bytesLabel(size) {
    if (size < 1024) return `${formatLocalizedNumber(size)} B`
    if (size < 1024 * 1024) return `${formatLocalizedNumber(size / 1024, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} KB`
    return `${formatLocalizedNumber(size / 1024 / 1024, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} MB`
  }

  /**
   * Formats an ISO date string for display.
   *
   * Returns a time (e.g. "2:30 PM") for today's dates and a short date
   * (e.g. "Jan 5") for older dates.
   *
   * @param {string} iso - An ISO 8601 date string.
   * @returns {string} A formatted date/time string.
   */
  formatDate(iso) {
    const d = new Date(iso), now = new Date()
    if (d.toDateString() === now.toDateString()) return formatLocalizedDate(d, {
      hour: '2-digit', minute: '2-digit', hour12: !this.$mailPreferences.use24HourClock,
    })
    return formatLocalizedDate(d, { month: 'short', day: 'numeric' })
  }

  /**
   * Navigates to the compose page, optionally storing prefill data in session
   * storage so the compose component can populate fields.
   *
   * @param {object} [prefill={}] - Key-value pairs to prefill (to, subject, body, etc.).
   */
  compose(prefill = {}) {
    if (Object.keys(prefill || {}).length > 0) {
      sessionStorage.setItem('caduceus_compose_prefill', JSON.stringify(prefill))
    } else {
      sessionStorage.removeItem('caduceus_compose_prefill')
    }
    this.navigate('/compose')
  }

  /**
   * Reads and clears the compose prefill data from session storage.
   *
   * @returns {Record<string, string>} The prefill object (empty object if nothing was stored).
   */
  consumeComposePrefill() {
    try {
      const stored = JSON.parse(sessionStorage.getItem('caduceus_compose_prefill') || '{}')
      sessionStorage.removeItem('caduceus_compose_prefill')
      const launch = composeLaunchPrefill(location.search)
      if (isManifestComposeLaunch(location.search)) history.replaceState(history.state, '', location.pathname)
      const value = stored && typeof stored === 'object' ? stored : {}
      return { .../** @type {Record<string, string>} */ (value), ...launch }
    } catch {
      sessionStorage.removeItem('caduceus_compose_prefill')
      return {}
    }
  }

  /**
   * Makes an authenticated API request to the CADUCEUS backend.
   *
   * Automatically attaches the Bearer token and handles 401 responses by
   * signing the user out and redirecting to the landing page.
   *
   * @async
   * @param {string} path - The API path (appended to the configured API URL).
   * @param {RequestInit} [options={}] - Fetch options (method, body, headers, etc.).
   * @returns {Promise<Response|null>} The fetch Response, or null if not
   *   authenticated or on auth failure.
   */
  async apiFetch(path, options = {}) {
    if (!this.$token) return null
    const headers = { 'Content-Type': 'application/json', ...options.headers }
    headers['Authorization'] = `Bearer ${this.$token}`
    const apiUrl = window.CADUCEUS_CONFIG?.apiUrl || ''
    const res = await fetch(apiUrl + path, { cache: 'no-store', ...options, headers })
    if (res.status === 401) { this.signOut(); return null }
    if (res.headers.get('content-type')?.includes('application/json')) {
      try {
        const data = await res.clone().json()
        if (data?.unauthorized) { this.signOut(); return null }
      } catch {}
    }
    return res
  }

  /**
   * Cached variant of {@link apiFetch} for LOW-SENSITIVITY, slowly-changing GET
   * endpoints (currently just `/users/me/preferences`). Backed by the
   * IndexedDB cache in `shared/scripts/fetch-cache.js` (a port of Tachyon
   * 26.23's TacBrowserCache).
   *
   * On a serverless backend this serves the last-known value when the API is
   * cold/slow/unreachable instead of dropping the data, and trims repeat
   * read invocations. Entries are namespaced per user; `signOut()` and the
   * no-session boot path wipe the store so a shared device never leaks one
   * account's data to the next.
   *
   * NEVER use this for message bodies, attachments, or anything sensitive —
   * keep those on `no-store` via {@link apiFetch}.
   *
   * @param {string} path
   * @param {{ cache?: 'cache-first'|'network-first'|'reload', invalidatePrefixes?: string[] }} [options]
   * @returns {Promise<Response | null>}
   */
  async apiFetchCached(path, { cache = 'network-first', invalidatePrefixes = [] } = {}) {
    if (!this.$token) return null
    const store = window._caduceusCache
    if (!store) return this.apiFetch(path)
    const apiUrl = window.CADUCEUS_CONFIG?.apiUrl || ''
    const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${this.$token}` }
    try {
      const res = await store.fetch(apiUrl + path, { headers }, { cache, key: this._cacheKey(path), invalidatePrefixes })
      if (res?.status === 401) { this.signOut(); return null }
      if (res?.headers.get('content-type')?.includes('application/json')) {
        try { if ((await res.clone().json())?.unauthorized) { this.signOut(); return null } } catch {}
      }
      return res
    } catch {
      return this.apiFetch(path)
    }
  }

  /**
   * Namespace a cache key by the signed-in user so accounts never share entries.
   * @param {string} path
   */
  _cacheKey(path) {
    return `caduceus:${(this.$email || 'anon').toLowerCase()}:${path}`
  }

  async cachePreferences(response) {
    if (!response?.ok) return
    await window._caduceusCache?.write?.(this._cacheKey('/users/me/preferences'), response)
  }

  /** Wipe every cached response for this origin (sign-out / shared-device safety). */
  _clearCache() {
    try { window._caduceusCache?.clearAll?.() } catch {}
  }

  /**
   * Signs the user out by clearing all auth state and navigating to the root.
   */
  signOut() {
    setScreenshotMode(false)
    clearTrustedExtensions()
    void window._caduceusUnreadBadge?.clear()
    delete window._caduceusContactsCache
    clearFontPreferences()
    clearNotificationSounds()
    clearMailPreferences()
    void window._caduceusKeyboard?.applyOverrides?.({})
    // Clearing local storage only stops this browser from presenting the
    // token. The session itself ends on the server, which also ends every
    // refresh grant resting on it, so a stolen copy stops working too.
    const idToken = sessionStorage.getItem('caduceus_id_token')
    if (idToken) {
      fetch(`${window.CADUCEUS_CONFIG?.apiUrl || ''}/auth/logout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ idToken }),
        keepalive: true,
      }).catch(() => {})
    }
    this.$token = null; this.$email = null; this.$name = null; this.$role = null; this.$domains = []; this.$mailboxNames = {}; this.$externalAccounts = []; this.$externalFolders = {}; this.$interfaceFont = initialFontPreferences.interfaceFont; this.$composerFont = initialFontPreferences.composerFont; this.$notificationSounds = initialNotificationSounds; this.$themePreferences = normalizeThemePreferences(DEFAULT_THEME_PREFERENCES); this.$mailPreferences = restoreMailPreferences(); this.isAuthenticated = false
    this.setUnreadBadgePreference({})
    for (const key of [
      'caduceus_token', 'caduceus_email', 'caduceus_name', 'caduceus_role',
      'caduceus_domains', 'caduceus_mailbox_names', 'caduceus_refresh', 'caduceus_id_token',
      'caduceus_interface_font', 'caduceus_composer_font',
      'caduceus_notification_sounds',
      'caduceus_mail_preferences',
    ]) sessionStorage.removeItem(key)
    this._clearCache()
    void clearOfflineWriteVault()
		this._offlineMailboxSync = null
    this.stopMailStream()
    this.stopActivityPolling()
    // Authentication state is owned by sessionStorage and the shell component.
    // A document navigation clears Tachyon's component-instance cache so an
    // authenticated route cannot retain its prior shell after sign-out.
    location.assign('/')
  }

  /**
   * Persists login data to reactive state and session storage, then redirects.
   *
   * Called by the login form.
   *
   * @param {object} data - The login response payload.
   * @param {string} data.token - The JWT session token.
   * @param {string} data.email - The authenticated user's email address.
   * @param {string} [data.accessToken] - The bearer token every request carries.
   * @param {string} [data.refreshToken] - Renews the access token.
   * @param {string} [data.idToken] - Ends the session on logout.
   * @param {string} [data.address] - The authenticated mailbox address.
   * @param {string} [data.domain] - The mailbox's domain.
   * @param {string} [data.name] - The authenticated user's display name.
   * @param {string} [data.role] - The user's role.
   * @param {string[]} [data.domains] - Allowed domains for the user.
   * @param {string} [returnTo='/inbox'] - The path to redirect to after login.
   */
  handleLogin(data, returnTo = '/inbox') {
    // The server issues one token set. `accessToken` is what every request
    // carries; `refreshToken` renews it without a second sign-in; `idToken` is
    // the only thing that can end the session on logout, so it is kept too.
    const token = data?.accessToken ?? data?.token
    const address = data?.address ?? data?.email
    if (!token || !address) { this.toast('Login failed: invalid response'); return }
    this.$token = token
    this.isAuthenticated = true
    this.$email = address
    this.$name = data.name || ''
    this.$role = data.role || ''
    this.$domains = data.domain ? [data.domain] : (data.domains || [])
    this.$mailboxNames = {}
    this.$externalAccounts = []
    this.$externalFolders = {}
    this.setUnreadBadgePreference({})
    const fonts = clearFontPreferences()
    this.$interfaceFont = fonts.interfaceFont
    this.$composerFont = fonts.composerFont
    this.$notificationSounds = clearNotificationSounds()
    sessionStorage.setItem('caduceus_token', token)
    sessionStorage.setItem('caduceus_email', address)
    sessionStorage.setItem('caduceus_name', data.name || '')
    sessionStorage.setItem('caduceus_role', data.role || '')
    sessionStorage.setItem('caduceus_domains', JSON.stringify(this.$domains))
    if (data.refreshToken) sessionStorage.setItem('caduceus_refresh', data.refreshToken)
    if (data.idToken) sessionStorage.setItem('caduceus_id_token', data.idToken)
    sessionStorage.removeItem('caduceus_mailbox_names')
    location.replace(returnTo)
  }
}

/**
 * Accent-color presets. Each entry maps the primary brand color plus a
 * lighter container shade (used as a tonal background behind selected
 * items) and the text color readable on that container.
 *
 * Adding a new accent: pick a `primary` hex, derive `container` as a tint
 * of the same hue at ~85% L*, and `onContainer` as a darker shade of the
 * primary readable on the container background.
 *
 * @type {Record<string, { label: string, primary: string, container: string, onContainer: string }>}
 */
const ACCENT_PRESETS = {
  teal:   { label: 'Teal (default)', primary: '#1f6f8b', container: '#d7edf4', onContainer: '#073544' },
  blue:   { label: 'Blue',           primary: '#2562d4', container: '#dbe7ff', onContainer: '#0a2d75' },
  purple: { label: 'Purple',         primary: '#6b3aa0', container: '#ebdcff', onContainer: '#341760' },
  pink:   { label: 'Pink',           primary: '#c2185b', container: '#ffd9e6', onContainer: '#5b0a28' },
  orange: { label: 'Orange',         primary: '#d97706', container: '#ffe4c2', onContainer: '#5a2e00' },
  green:  { label: 'Green',          primary: '#2e7d32', container: '#cfe9d0', onContainer: '#0e3911' },
  red:    { label: 'Red',            primary: '#c0392b', container: '#ffd9d4', onContainer: '#5a120a' },
  slate:  { label: 'Slate',          primary: '#475569', container: '#dbe3ee', onContainer: '#1c2735' },
}
