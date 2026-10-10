// @ts-check

import { sanitizeSignatureHTML, signatureHTMLFromClipboard } from '../../../shared/scripts/composer-signatures.js'
import {
  COMPOSER_FONT_OPTIONS,
  INTERFACE_FONT_OPTIONS,
  composerFontStack,
  interfaceFontStack,
  normalizeComposerFont,
  normalizeInterfaceFont,
} from '../../../shared/scripts/font-preferences.js'
import {
  DEFAULT_NOTIFICATION_SOUNDS,
  SOUND_EVENTS,
  SOUND_OPTIONS,
  isSoundEventId,
  isSoundId,
  normalizeNotificationSounds,
} from '../../../shared/scripts/notification-sounds.js'
import {
  DARK_THEME_OPTIONS,
  DEFAULT_THEME_PREFERENCES,
  LIGHT_THEME_OPTIONS,
  THEME_OPTIONS,
  normalizeThemePreferences,
} from '../../../shared/scripts/theme-preferences.js'
import { TRANSLATION_LANGUAGES } from '../../../shared/scripts/on-device-translation.js'
import {
  loadTranslationPreferences,
  resetTranslationPreferences,
  setTranslationPreference,
} from '../../../shared/scripts/translation-preferences.js'
import { normalizeUnreadBadgePreference } from '../../../shared/scripts/unread-badge.js'
import {
  DEFAULT_MAIL_PREFERENCES,
  normalizeMailPreferences,
} from '../../../shared/scripts/mail-preferences.js'
import {
  accountActions,
  accountProgressLabel,
  connectionDraft,
  normalizeExternalAccounts,
  normalizeProviderCatalog,
} from '../../../shared/scripts/external-accounts-ui.js'
import {
  INTERFACE_LOCALE_OPTIONS,
  formatDate as formatLocalizedDate,
  normalizeInterfaceLocale,
  translate,
} from '../../../shared/scripts/localization.js'

const CATEGORY_ROLES = Object.freeze([
  { id: 'inbox', label: 'Inbox' },
  { id: 'sent', label: 'Sent mail' },
  { id: 'drafts', label: 'Drafts' },
  { id: 'spam', label: 'Spam' },
  { id: 'archive', label: 'Archive' },
  { id: 'trash', label: 'Trash' },
])

/** @param {string | number | Date | null | undefined} value */
function dateLabel(value) {
  return formatLocalizedDate(value)
}

/** @param {string | number | Date | null | undefined} value */
function dateTimeLabel(value) {
  return formatLocalizedDate(value, { dateStyle: 'medium', timeStyle: 'short' })
}

/**
 * Settings / administration panel component.
 *
 * Provides a tabbed interface for managing the Caduceus email system:
 * - **Domains** — list of configured email domains
 * - **Users** — admin-only user management and assisted recovery
 * - **Rules** — server-side email processing rules with conditions and actions
 * - **MFA Devices** — WebAuthn passkey provisioning and removal
 * - **Notifications** — Web Push subscription management (enable / disable)
 * - **Signatures** — per-domain email signature management
 * - **Templates** — reusable rich composer templates and editable blanks
 *
 * All data is loaded in parallel on mount via `Promise.all`. The MFA setup
 * wizard preserves its state across re-renders using a window-level cache.
 *
 */
export default class {
  static __tachyonOnMount = ['primeShortcutEditor', 'loadAll', 'bindSignatureEditor', 'bindCategoryMappingDrafts', 'bindMailPreferenceDraft']

  /** @type {Array<Record<string, any>>} */
  $domains = []; /** @type {Array<Record<string, any>>} */
  $users = []; /** @type {Array<Record<string, any>>} */
  $rules = []; /** @type {Array<Record<string, any>>} */
  $mfaDevices = []; /** @type {Array<Record<string, any>>} */
  $pushSubscriptions = []; /** @type {boolean} */
  $loading = true
  /** @type {Array<Record<string, any>>} Redacted external mailbox projections. */
  $externalAccounts = []
  /** @type {Array<Record<string, any>>} Server-owned provider catalog. */
  $providerCatalog = []
  /** @type {boolean} */
  $accountSetupOpen = false
  /** @type {boolean} Whether the explicit first-run account guide is active. */
  $accountOnboarding = false
  /** @type {'address'|'credentials'|'oauth'} */
  $accountSetupStep = 'address'
  /** @type {string} */
  $accountAddress = ''
  /** @type {Record<string, any>} Non-secret discovered connection fields. */
  $accountConnectionDraft = connectionDraft({}, '')
  /** @type {string} */
  $accountSetupStatus = ''
  /** @type {string} */
  $accountSetupError = ''
  /** @type {boolean} */
  $accountSetupBusy = false
  /** @type {string} */
  $accountMutationId = ''
  /** @type {string} */
  $accountReconnectId = ''
  /** @type {string} */
  $accountRemoveId = ''
  /** @type {string} */
  $accountDiagnosticsId = ''
  /** @type {Record<string, any>} */
  $accountDiagnostics = {}
  /** @type {string} */
  $externalAccountsError = ''
  /** @type {string} Stable result of a Microsoft authorization redirect. */
  $externalOAuthNotice = ''
  /** @type {string} */
  $primaryEmail = ''
  /** @type {string[]} */
  $aliases = []
  /** @type {string} */
  $newAlias = ''
  /** @type {string} */
  $aliasError = ''
  /** @type {boolean} */
  $aliasSaving = false

  /**
   * Selected calendar-sharing visibility (private | busy | full). Mirrors
   * the server-side user record; loaded on mount and re-pulled after every
   * change so the radio reflects the persisted state.
   * @type {string}
   */
  $calendarVisibility = 'private'
  /** @type {Record<string, string>} Personal labels keyed by assigned domain. */
  $mailboxNames = {}
  /** @type {boolean} */
  $mailboxSaving = false
  /** @type {string} */
  $mailboxError = ''
  /** @type {Array<Record<string, any>>} Exact-domain physical category catalogs. */
  $categoryAccounts = []
  /** @type {Record<string, Record<string, string>>} */
  $categoryMappings = {}
  /** @type {Record<string, string>} Native category edits awaiting one atomic save. */
  _categoryMappingDrafts = {}
  /** @type {boolean} */
  $categorySaving = false
  /** @type {string} */
  $categoryError = ''
  /** @type {boolean} Whether the owner's MCP capability may authenticate. */
  $mcpEnabled = false
  /** @type {'read-only'|'read-write'|'read-write-send'} */
  $mcpAccessLevel = 'read-only'
  /** @type {string[]} Exact mailbox domains exposed to MCP. */
  $mcpAllowedDomains = []
  /** @type {Record<string, string[]>} Physical folders hidden from every MCP tool. */
  $mcpExcludedFolders = {}
  /** @type {boolean} */
  $mcpHasToken = false
  /** @type {string} One-time plaintext capability returned by the last rotation. */
  $mcpToken = ''
  /** @type {string} */
  $mcpTokenCreatedAt = ''
  /** @type {string} */
  $mcpEndpointPath = '/mcp'
  /** @type {Array<Record<string, any>>} */
  $mcpAudit = []
  /** @type {boolean} Prevent edits while the persisted scope is arriving. */
  $mcpLoading = true
  /** @type {boolean} */
  $mcpSaving = false
  /** @type {string} */
  $mcpError = ''
  /** @type {'block'|'allow'} Default policy for ordinary remote email images. */
  $remoteImagePolicy = 'block'
  /** @type {string[]} Exact senders whose images may load automatically. */
  $trustedImageSenders = []
  /** @type {boolean} Allow opt-in public GitHub lookup for owned contacts. */
  $githubProfileEnrichment = false
  /** @type {string} Catalog identifier for application chrome and controls. */
  $interfaceFont = 'plex-sans'
  /** @type {string} Catalog identifier for authored message text. */
  $composerFont = 'interface'
  /** @type {ReturnType<typeof normalizeInterfaceLocale>} */
  $interfaceLocale = 'auto'
  /** @type {boolean} */
  $localeSaving = false
  /** @type {string} */
  $localeError = ''
  /** @type {boolean} */
  $fontSaving = false
  /** @type {string} */
  $fontError = ''
  /** @type {boolean} */
  $privacySaving = false
  /** @type {string[]} Languages translated automatically on this device. */
  $translationAutomaticLanguages = []
  /** @type {string[]} Languages whose translation offer is hidden on this device. */
  $translationDisabledLanguages = []
  /** @type {string} */
  $privacyError = ''
  /** @type {{ activeTheme: 'automatic'|'light'|'dark'|'darkside'|'less-is-more'|'taiga'|'ubuntu', lightTheme: 'light'|'less-is-more'|'taiga'|'ubuntu', darkTheme: 'dark'|'darkside' }} */
  $themePreferences = normalizeThemePreferences(DEFAULT_THEME_PREFERENCES)
  /** @type {boolean} */
  $themeSaving = false
  /** @type {string} */
  $themeError = ''
  /** @type {Array<Record<string, any>>} Explicitly trusted owner packages. */
  $extensions = []
  /** @type {Record<string, any> | null} Immutable package projection awaiting trust. */
  $extensionInspection = null
  /** @type {Record<string, any> | null} Original package paired with the inspected digest. */
  _extensionBundle = null
  /** @type {boolean} */
  $extensionBusy = false
  /** @type {string} */
  $extensionError = ''
  /** @type {ReturnType<typeof normalizeMailPreferences>} */
  $mailPreferences = normalizeMailPreferences(DEFAULT_MAIL_PREFERENCES)
  /** @type {boolean} */
  $mailPreferencesSaving = false
  /** @type {string} */
  $mailPreferencesError = ''
  /** @type {string} */
  $mailPreferencesStatus = ''
  /** @type {Array<Record<string, any>>} */
  $shortcutCatalog = []
  /** @type {Record<string, string>} Effective editable bindings keyed by command. */
  $shortcutDraft = {}
  /** @type {Record<string, string>} Compact server-owned override map. */
  $keyboardShortcutOverrides = {}
  /** @type {boolean} Prevent editing defaults before account overrides arrive. */
  $shortcutLoading = true
  /** @type {boolean} */
  $shortcutSaving = false
  /** @type {string} */
  $shortcutError = ''
  /** @type {string} */
  $shortcutStatus = ''
  /** @type {number} Reactive invalidation token for shell-owned appearance state. */
  accentRevision = 0

  /**
   * Flat list of accent presets for rendering in the Appearance section.
   * Sourced from the app shell's exposed `window._caduceus.accentPresets`
   * (the global is registered in `pages/tac.js`).
   *
   * @returns {Array<{ key: string, label: string, primary: string }>}
   */
  get accentOptions() {
    const presets = window._caduceus?.accentPresets || {}
    return Object.entries(presets).map(([key, val]) => ({ key, ...val }))
  }

  /** @returns {string} */
  get currentAccent() { return window._caduceus?.accentColor || 'teal' }
  /** @returns {string} */
  get currentAccentLabel() {
    const k = this.currentAccent
    return window._caduceus?.accentPresets?.[k]?.label || k
  }

  get interfaceFontOptions() { return INTERFACE_FONT_OPTIONS }
  get composerFontOptions() { return COMPOSER_FONT_OPTIONS }
  get themeOptions() { return THEME_OPTIONS }
  get lightThemeOptions() { return LIGHT_THEME_OPTIONS }
  get darkThemeOptions() { return DARK_THEME_OPTIONS }
  get interfaceLocaleOptions() {
    return INTERFACE_LOCALE_OPTIONS.map(option => ({
      ...option,
      label: option.label || translate(option.labelKey || 'locale.automatic'),
    }))
  }
  t(id, values = {}) { return translate(id, values) }
  get categoryRoles() { return CATEGORY_ROLES }
  get mailPreferenceSenderRows() {
    return this.senderAddresses.map(address => ({
      address,
      cc: (this.$mailPreferences.autoCcBySender[address] || []).join(', '),
      bcc: (this.$mailPreferences.autoBccBySender[address] || []).join(', '),
    }))
  }
  get mailPreferenceDomainRows() {
    return this.mailboxDomains.map(domain => ({
      domain,
      enabled: this.$mailPreferences.findInMailDisabledDomains.includes(domain),
    }))
  }
  get spellcheckLanguageOptions() {
    return [
      { id: '', label: 'Device default' }, { id: 'af', label: 'Afrikaans' },
      { id: 'bg', label: 'Bulgarian' }, { id: 'ca', label: 'Catalan' },
      { id: 'cs', label: 'Czech' }, { id: 'da', label: 'Danish' },
      { id: 'de', label: 'German' }, { id: 'el', label: 'Greek' },
      { id: 'en-AU', label: 'English (Australia)' }, { id: 'en-CA', label: 'English (Canada)' },
      { id: 'en-GB', label: 'English (UK)' }, { id: 'en-US', label: 'English (US)' },
      { id: 'es', label: 'Spanish' }, { id: 'et', label: 'Estonian' },
      { id: 'fo', label: 'Faroese' }, { id: 'fr', label: 'French' },
      { id: 'he', label: 'Hebrew' }, { id: 'hi', label: 'Hindi' },
      { id: 'hr', label: 'Croatian' }, { id: 'hu', label: 'Hungarian' },
      { id: 'it', label: 'Italian' }, { id: 'ko', label: 'Korean' },
      { id: 'lt', label: 'Lithuanian' }, { id: 'lv', label: 'Latvian' },
      { id: 'nb', label: 'Norwegian Bokmal' }, { id: 'nl', label: 'Dutch' },
      { id: 'pl', label: 'Polish' }, { id: 'pt', label: 'Portuguese' },
      { id: 'pt-BR', label: 'Portuguese (Brazil)' }, { id: 'ro', label: 'Romanian' },
      { id: 'ru', label: 'Russian' }, { id: 'sk', label: 'Slovak' },
      { id: 'sl', label: 'Slovenian' }, { id: 'sq', label: 'Albanian' },
      { id: 'sr', label: 'Serbian' }, { id: 'sv', label: 'Swedish' },
      { id: 'tr', label: 'Turkish' }, { id: 'uk', label: 'Ukrainian' },
      { id: 'vi', label: 'Vietnamese' },
    ]
  }
  get categoryPaths() {
    return [...new Set(this.$categoryAccounts.flatMap(account => (
      Array.isArray(account?.categories) ? account.categories.map(category => category.path) : []
    )))].sort()
  }
  get categoryRows() {
    return this.$categoryAccounts.flatMap(account => CATEGORY_ROLES.map(role => ({
      domain: account.domain,
      role: role.id,
      label: role.label,
      value: account?.mappings?.[role.id] || role.id,
    })))
  }
  get mcpDomainRows() {
    return this.$categoryAccounts.map(account => ({
      domain: account.domain,
      label: this.mailboxLabel(account.domain),
      allowed: this.$mcpAllowedDomains.includes(account.domain),
    }))
  }
  get mcpFolderRows() {
    return this.$categoryAccounts.flatMap(account => (
      Array.isArray(account?.categories) ? account.categories : []
    ).map(category => ({
      domain: account.domain,
      path: category.path,
      role: category.role || '',
      accountAllowed: this.$mcpAllowedDomains.includes(account.domain),
      excluded: (this.$mcpExcludedFolders[account.domain] || []).includes(category.path),
    })))
  }
  get mcpAuditRows() {
    return this.$mcpAudit.map(event => ({
      ...event,
      createdAtLabel: dateTimeLabel(event.createdAt),
      parameterLabel: Array.isArray(event.parameterKeys) && event.parameterKeys.length
        ? event.parameterKeys.join(', ')
        : 'none',
    }))
  }
  get mcpEndpoint() {
    try { return new URL(this.$mcpEndpointPath || '/mcp', window.location.origin).href } catch { return '/mcp' }
  }
  get mcpTokenCreatedAtLabel() { return dateTimeLabel(this.$mcpTokenCreatedAt) }
  get activeTheme() { return this.$themePreferences.activeTheme }
  get automaticThemeSelected() { return this.activeTheme === 'automatic' }
  get shortcutGroups() {
    const groups = []
    for (const action of this.$shortcutCatalog) {
      let group = groups.find(candidate => candidate.label === action.group)
      if (!group) { group = { label: action.group, rows: [] }; groups.push(group) }
      const binding = this.$shortcutDraft[action.id] || action.binding
      group.rows.push({
        ...action,
        binding,
        inputBinding: binding === 'off' ? '' : binding,
        defaultLabel: window._caduceusKeyboard?.displayBinding?.(action.binding) || action.binding,
      })
    }
    return groups
  }
  get translationAutomaticRows() {
    return this.$translationAutomaticLanguages.map(id => ({ id, label: TRANSLATION_LANGUAGES[id] || id }))
  }
  get translationDisabledRows() {
    return this.$translationDisabledLanguages.map(id => ({ id, label: TRANSLATION_LANGUAGES[id] || id }))
  }

  /** @param {{ colors?: string[] }} theme */
  themePreviewStyle(theme) {
    const colors = Array.isArray(theme?.colors) ? theme.colors : []
    return `--theme-preview-1:${colors[0] || '#fff'};--theme-preview-2:${colors[1] || '#eee'};--theme-preview-3:${colors[2] || '#777'}`
  }

  /** @param {string} id */
  interfaceFontPreview(id) { return `font-family:${interfaceFontStack(id)}` }

  /** @param {string} id */
  composerFontPreview(id) { return `font-family:${composerFontStack(id)}` }

  /** @returns {Array<Record<string, any>>} */
  get passkeyDevices() {
    return this.$mfaDevices
      .filter(device => device.type === 'webauthn' || device.credentialId)
      .map(device => ({ ...device, createdAtLabel: dateLabel(device.createdAt) }))
  }

  /** @returns {Array<Record<string, any>>} */
  get totpDevices() { return this.$mfaDevices.filter(device => device.type === 'totp') }

  /** @returns {Array<Record<string, any>>} */
  get pushSubscriptions() {
    return this.$pushSubscriptions.map(subscription => ({
      ...subscription,
      updatedAtLabel: dateLabel(subscription.updatedAt),
    }))
  }

  /** @returns {Array<Record<string, any>>} */
  get usersWithRecovery() {
    return this.$users.map(user => ({
      ...user,
      adminRecovery: this.$adminRecoveryCodes[user.email] || null,
    }))
  }

  toggleAddUser() { this.$showAddUser = !this.$showAddUser }

  /**
   * Forward an accent-color choice to the app-shell, which will persist
   * (localStorage) and apply (--ms-primary override) on the document.
   * Invalidates this companion so selected-state styling updates.
   *
   * @param {Event} _event
   * @param {string} key
   */
  selectAccent(_event, key) {
    window._caduceus?.setAccent?.(key)
    this.accentRevision += 1
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /** @param {Event} _event @param {{ id?: string } | string} theme */
  async selectTheme(_event, theme) {
    const activeTheme = typeof theme === 'string' ? theme : theme?.id
    await this.saveThemePreferences({ activeTheme })
  }

  /** @param {Event} event @param {'light'|'dark'} slot */
  async selectAutomaticTheme(event, slot) {
    const target = event?.currentTarget instanceof HTMLSelectElement
      ? event.currentTarget
      : document.querySelector(`[data-automatic-theme="${slot}"]`)
    const value = target instanceof HTMLSelectElement ? target.value : ''
    await this.saveThemePreferences(slot === 'light' ? { lightTheme: value } : { darkTheme: value })
  }

  /** @param {Record<string, unknown>} patch */
  async saveThemePreferences(patch) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$themeSaving) return
    const previous = this.$themePreferences
    const next = normalizeThemePreferences({ ...previous, ...patch })
    this.$themePreferences = next
    this.$themeSaving = true
    this.$themeError = ''
    window._caduceus?.setThemePreferences?.(next)
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ themePreferences: next }),
      })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save theme.')
      this.$themePreferences = normalizeThemePreferences(data?.themePreferences)
      window._caduceus?.setThemePreferences?.(this.$themePreferences)
      window._caduceus?.toast('Theme preference saved.')
    } catch (error) {
      this.$themePreferences = previous
      window._caduceus?.setThemePreferences?.(previous)
      this.$themeError = error instanceof Error ? error.message : 'Could not save theme.'
    } finally {
      this.$themeSaving = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      this.syncThemeControls()
    }
  }

  /** Persist and immediately apply one fixed interface locale choice. */
  async selectInterfaceLocale(event) {
    const target = event?.currentTarget instanceof HTMLSelectElement
      ? event.currentTarget
      : document.querySelector('select[data-interface-locale]')
    if (!(target instanceof HTMLSelectElement) || this.$localeSaving) return
    const previous = this.$interfaceLocale
    const next = normalizeInterfaceLocale(target.value)
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$interfaceLocale = next
    this.$localeSaving = true
    this.$localeError = ''
    window._caduceus?.setInterfaceLocale?.(next)
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT', body: JSON.stringify({ interfaceLocale: next }),
      })
      const cacheCopy = response?.clone()
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || translate('locale.saveError'))
      this.$interfaceLocale = normalizeInterfaceLocale(data?.interfaceLocale)
      window._caduceus?.setInterfaceLocale?.(this.$interfaceLocale)
      if (cacheCopy) await window._caduceus?.cachePreferences?.(cacheCopy)
      window._caduceus?.toast(translate('locale.saved'))
    } catch (error) {
      this.$interfaceLocale = previous
      window._caduceus?.setInterfaceLocale?.(previous)
      this.$localeError = error instanceof Error ? error.message : translate('locale.saveError')
    } finally {
      this.$localeSaving = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      const control = document.querySelector('select[data-interface-locale]')
      if (control instanceof HTMLSelectElement) control.value = this.$interfaceLocale
    }
  }

  syncThemeControls() {
    const light = /** @type {HTMLSelectElement | null} */ (document.querySelector('[data-automatic-theme="light"]'))
    const dark = /** @type {HTMLSelectElement | null} */ (document.querySelector('[data-automatic-theme="dark"]'))
    if (light) light.value = this.$themePreferences.lightTheme
    if (dark) dark.value = this.$themePreferences.darkTheme
  }

  /**
   * Load the current calendar-visibility preference from the server so the
   * radio reflects the persisted state on first paint. Called from the
   * existing init() pipeline via {@link loadPreferences}.
   */
  async loadPreferences() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$shortcutLoading = false; return }
    try {
      const res = await apiFetch('/users/me/preferences')
      if (!res?.ok) return
      const prefs = await res.json()
      if (prefs?.calendarVisibility) this.$calendarVisibility = prefs.calendarVisibility
      this.$mailboxNames = prefs?.mailboxNames && typeof prefs.mailboxNames === 'object' ? prefs.mailboxNames : {}
      this.$categoryMappings = prefs?.categoryMappings && typeof prefs.categoryMappings === 'object' ? prefs.categoryMappings : {}
      this.$remoteImagePolicy = prefs?.remoteImagePolicy === 'allow' ? 'allow' : 'block'
      this.$trustedImageSenders = Array.isArray(prefs?.trustedImageSenders) ? prefs.trustedImageSenders : []
      this.$githubProfileEnrichment = prefs?.githubProfileEnrichment === true
      this.$interfaceFont = normalizeInterfaceFont(prefs?.interfaceFont)
      this.$composerFont = normalizeComposerFont(prefs?.composerFont)
      this.$interfaceLocale = normalizeInterfaceLocale(prefs?.interfaceLocale)
      this.$notificationSounds = normalizeNotificationSounds(prefs?.notificationSounds)
      this.$themePreferences = normalizeThemePreferences(prefs?.themePreferences)
      this.$mailPreferences = normalizeMailPreferences(prefs?.mailPreferences)
      await this.loadShortcutEditor(prefs?.keyboardShortcutOverrides)
      const unreadBadge = normalizeUnreadBadgePreference(prefs, this.mailboxDomains)
      this.$unreadBadgeScope = unreadBadge.scope
      this.$unreadBadgeDomain = unreadBadge.domain
      const notificationPolicy = this.normalizeNewMailNotificationPolicy(prefs)
      this.$newMailNotificationDomains = notificationPolicy.domains
      this.$newMailNotificationExternalAccounts = notificationPolicy.externalAccounts
      this.$newMailNotificationPrivacy = notificationPolicy.privacy
      this.$newMailNotificationActions = notificationPolicy.actions
      window._caduceus?.setMailboxNames?.(this.$mailboxNames)
      window._caduceus?.setFontPreferences?.({
        interfaceFont: this.$interfaceFont,
        composerFont: this.$composerFont,
      })
      window._caduceus?.setInterfaceLocale?.(this.$interfaceLocale)
      window._caduceus?.setNotificationSounds?.(this.$notificationSounds)
      window._caduceus?.setThemePreferences?.(this.$themePreferences)
      window._caduceus?.setUnreadBadgePreference?.(prefs)
      window._caduceus?.setMailPreferences?.(this.$mailPreferences)
    } catch {
      // Preferences are non-critical — silently keep the default.
    } finally {
      this.$shortcutLoading = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  async loadShortcutEditor(overrides) {
    const source = overrides && typeof overrides === 'object' && !Array.isArray(overrides) ? overrides : {}
    const applied = await window._caduceusKeyboard?.applyOverrides?.(source)
    this.$keyboardShortcutOverrides = applied?.ok ? { ...applied.overrides } : {}
    this.$shortcutCatalog = window._caduceusKeyboard?.getCatalog?.() || []
    this.$shortcutDraft = window._caduceusKeyboard?.getBindings?.() || Object.fromEntries(
      this.$shortcutCatalog.map(action => [action.id, action.binding]),
    )
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  async primeShortcutEditor() {
    await window._caduceusKeyboard?.ready?.()
    await this.loadShortcutEditor(window._caduceusKeyboard?.getOverrides?.() || {})
  }

  /** @param {Event} event */
  captureShortcutBinding(event) {
    if (!(event instanceof Event)) return
    const target = event.target instanceof HTMLInputElement
      ? event.target
      : event.currentTarget instanceof HTMLInputElement ? event.currentTarget : null
    if (!target) return
    const actionId = target.getAttribute('data-shortcut-action') || ''
    if (!actionId) return
    this.$shortcutDraft = { ...this.$shortcutDraft, [actionId]: target.value.trim() || 'off' }
    this.$shortcutError = ''
    this.$shortcutStatus = ''
  }

  async saveKeyboardShortcuts() {
    if (this.$shortcutSaving) return
    const draft = { ...this.$shortcutDraft }
    for (const input of document.querySelectorAll('[data-shortcut-action]')) {
      if (!(input instanceof HTMLInputElement)) continue
      const actionId = input.getAttribute('data-shortcut-action') || ''
      if (actionId) draft[actionId] = input.value.trim() || 'off'
    }
    this.$shortcutDraft = draft
    const validation = window._caduceusKeyboard?.validateOverrides?.(this.$shortcutDraft)
    if (!validation?.ok) {
      this.$shortcutError = validation?.error || 'Keyboard shortcuts are invalid.'
      this.$shortcutStatus = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      return
    }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$shortcutSaving = true
    this.$shortcutError = ''
    this.$shortcutStatus = ''
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ keyboardShortcutOverrides: validation.overrides }),
      })
      const cacheCopy = response?.clone()
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save keyboard shortcuts.')
      if (cacheCopy) await window._caduceus?.cachePreferences?.(cacheCopy)
      await this.loadShortcutEditor(data?.keyboardShortcutOverrides)
      this.$shortcutStatus = 'Keyboard shortcuts saved.'
      window._caduceus?.toast('Keyboard shortcuts saved.')
    } catch (error) {
      this.$shortcutError = error instanceof Error ? error.message : 'Could not save keyboard shortcuts.'
    } finally {
      this.$shortcutSaving = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  async resetKeyboardShortcuts() {
    this.$shortcutDraft = Object.fromEntries(this.$shortcutCatalog.map(action => [action.id, action.binding]))
    await this.saveKeyboardShortcuts()
  }

  /** Preserve every native preference control before Tachyon reconciles one change. */
  bindMailPreferenceDraft() {
    const previous = window._caduceusMailPreferenceDraftHandler
    if (previous) document.removeEventListener('change', previous, true)
    window._caduceusMailPreferenceDraftHandler = event => {
      if (!(event.target instanceof Element) || !event.target.closest('[data-mail-preferences-settings]')) return
      this.$mailPreferences = this._mailPreferencesFromControls()
    }
    document.addEventListener('change', window._caduceusMailPreferenceDraftHandler, true)
  }

  _mailPreferencesFromControls() {
    const select = (name) => /** @type {HTMLSelectElement | null} */ (document.querySelector(`[data-mail-preference="${name}"]`))?.value
    const checked = (name) => Boolean(/** @type {HTMLInputElement | null} */ (document.querySelector(`[data-mail-preference="${name}"]`))?.checked)
    const number = (name) => Number(select(name))
    /** @type {Record<string, string[]>} */
    const autoCcBySender = {}
    /** @type {Record<string, string[]>} */
    const autoBccBySender = {}
    for (const input of document.querySelectorAll('[data-auto-recipient-sender]')) {
      if (!(input instanceof HTMLInputElement)) continue
      const sender = input.dataset.autoRecipientSender || ''
      const kind = input.dataset.autoRecipientKind
      const recipients = input.value.split(',').map(value => value.trim()).filter(Boolean)
      if (recipients.length && kind === 'cc') autoCcBySender[sender] = recipients
      if (recipients.length && kind === 'bcc') autoBccBySender[sender] = recipients
    }
    const findInMailDisabledDomains = [...document.querySelectorAll('[data-find-in-mail-domain]')]
      .filter(input => input instanceof HTMLInputElement && input.checked)
      .map(input => /** @type {HTMLInputElement} */ (input).value)
    return normalizeMailPreferences({
      workspaceLayout: select('workspaceLayout'), use24HourClock: checked('use24HourClock'),
      interfaceScale: number('interfaceScale'), showUnreadForAllFolders: checked('showUnreadForAllFolders'),
      markAsReadDelay: number('markAsReadDelay'), detailedHeaders: checked('detailedHeaders'),
      detailedNames: checked('detailedNames'), restrictMessageWidth: checked('restrictMessageWidth'),
      backspaceAction: select('backspaceAction'), swipeEnabled: checked('swipeEnabled'),
      conversationOrder: select('conversationOrder'), emailRenderMode: select('emailRenderMode'),
      richText: checked('richText'), spellcheck: checked('spellcheck'),
      spellcheckLanguage: select('spellcheckLanguage'), defaultSendAction: select('defaultSendAction'),
      defaultReplyAction: select('defaultReplyAction'), undoSendDelay: number('undoSendDelay'),
      defaultFrom: select('defaultFrom'), autoCcBySender, autoBccBySender,
      displayAttachmentPreviews: checked('displayAttachmentPreviews'),
      unsnoozeToTop: checked('unsnoozeToTop'), findInMailDisabledDomains,
    })
  }

  /** Read every native mail preference control and persist one atomic profile. */
  async saveMailPreferences() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$mailPreferencesSaving) return
    const next = this._mailPreferencesFromControls()
    this.$mailPreferencesSaving = true
    this.$mailPreferencesError = ''
    this.$mailPreferencesStatus = ''
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT', body: JSON.stringify({ mailPreferences: next }),
      })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save mail preferences.')
      this.$mailPreferences = normalizeMailPreferences(data?.mailPreferences)
      window._caduceus?.setMailPreferences?.(this.$mailPreferences)
      this.$mailPreferencesStatus = 'Mail preferences saved.'
      window._caduceus?.toast('Mail preferences saved.')
    } catch (error) {
      this.$mailPreferencesError = error instanceof Error ? error.message : 'Could not save mail preferences.'
    } finally {
      this.$mailPreferencesSaving = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** Load the least-privilege MCP configuration and its redacted local audit. */
  async loadMcp() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$mcpLoading = false; return }
    this.$mcpLoading = true
    try {
      const [configResponse, auditResponse] = await Promise.all([
        apiFetch('/mcp/config'),
        apiFetch('/mcp/audit'),
      ])
      if (configResponse?.ok) {
        const config = await configResponse.json()
        this.$mcpEnabled = config?.enabled === true
        this.$mcpAccessLevel = ['read-only', 'read-write', 'read-write-send'].includes(config?.accessLevel)
          ? config.accessLevel
          : 'read-only'
        this.$mcpAllowedDomains = Array.isArray(config?.allowedDomains) ? config.allowedDomains : []
        this.$mcpExcludedFolders = config?.excludedFolders && typeof config.excludedFolders === 'object'
          ? config.excludedFolders
          : {}
        this.$mcpHasToken = config?.hasToken === true
        this.$mcpTokenCreatedAt = config?.tokenCreatedAt || ''
        this.$mcpEndpointPath = config?.endpoint || '/mcp'
      }
      this.$mcpAudit = auditResponse?.ok ? await auditResponse.json() : []
    } catch {
      this.$mcpError = 'Could not load MCP access settings.'
    } finally {
      this.$mcpLoading = false
    }
  }

  setMcpEnabled() { this.$mcpEnabled = !this.$mcpEnabled }

  /** @param {Event} _event @param {string} value */
  setMcpAccessLevel(_event, value) {
    if (['read-only', 'read-write', 'read-write-send'].includes(value)) {
      this.$mcpAccessLevel = /** @type {'read-only'|'read-write'|'read-write-send'} */ (value)
    }
  }

  /** @param {Event} _event @param {string} domain */
  toggleMcpDomain(_event, domain) {
    const checked = !this.$mcpAllowedDomains.includes(domain)
    this.$mcpAllowedDomains = checked
      ? [...new Set([...this.$mcpAllowedDomains, domain])]
      : this.$mcpAllowedDomains.filter(value => value !== domain)
    if (!checked && this.$mcpExcludedFolders[domain]) {
      const next = { ...this.$mcpExcludedFolders }
      delete next[domain]
      this.$mcpExcludedFolders = next
    }
  }

  /** @param {Event} _event @param {{domain:string,path:string}} row */
  toggleMcpFolder(_event, row) {
    const current = this.$mcpExcludedFolders[row.domain] || []
    const excluded = !current.includes(row.path)
    const folders = excluded
      ? [...new Set([...current, row.path])]
      : current.filter(path => path !== row.path)
    const next = { ...this.$mcpExcludedFolders }
    if (folders.length) next[row.domain] = folders
    else delete next[row.domain]
    this.$mcpExcludedFolders = next
  }

  async saveMcpConfig() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$mcpSaving) return
    this.$mcpSaving = true
    this.$mcpError = ''
    try {
      const response = await apiFetch('/mcp/config', {
        method: 'PUT',
        body: JSON.stringify({
          enabled: this.$mcpEnabled,
          accessLevel: this.$mcpAccessLevel,
          allowedDomains: this.$mcpAllowedDomains,
          excludedFolders: this.$mcpExcludedFolders,
        }),
      })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save MCP access.')
      this.$mcpEnabled = data.enabled === true
      this.$mcpAllowedDomains = data.allowedDomains || []
      this.$mcpExcludedFolders = data.excludedFolders || {}
      window._caduceus?.toast('MCP access saved.')
    } catch (error) {
      this.$mcpError = error instanceof Error ? error.message : 'Could not save MCP access.'
    } finally {
      this.$mcpSaving = false
    }
  }

  async rotateMcpToken() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$mcpSaving) return
    this.$mcpSaving = true
    this.$mcpError = ''
    try {
      const response = await apiFetch('/mcp/token', { method: 'POST' })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not generate the MCP token.')
      this.$mcpToken = data.token || ''
      this.$mcpHasToken = true
      this.$mcpTokenCreatedAt = data.createdAt || ''
      window._caduceus?.toast('MCP token rotated. Copy it now.')
    } catch (error) {
      this.$mcpError = error instanceof Error ? error.message : 'Could not generate the MCP token.'
    } finally {
      this.$mcpSaving = false
    }
  }

  async revokeMcpToken() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$mcpSaving) return
    this.$mcpSaving = true
    this.$mcpError = ''
    try {
      const response = await apiFetch('/mcp/token', { method: 'DELETE' })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not revoke the MCP token.')
      this.$mcpToken = ''
      this.$mcpHasToken = false
      this.$mcpTokenCreatedAt = ''
      window._caduceus?.toast(data.revoked ? 'MCP token revoked.' : 'No active MCP token.')
    } catch (error) {
      this.$mcpError = error instanceof Error ? error.message : 'Could not revoke the MCP token.'
    } finally {
      this.$mcpSaving = false
    }
  }

  /** @param {Event} _event @param {string} value @param {string} label */
  async copyMcpValue(_event, value, label) {
    try {
      await navigator.clipboard.writeText(value)
      window._caduceus?.toast(`${label} copied.`)
    } catch {
      this.$mcpError = `Could not copy ${label.toLowerCase()}.`
    }
  }

  async clearMcpAudit() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$mcpSaving) return
    this.$mcpSaving = true
    try {
      const response = await apiFetch('/mcp/audit', { method: 'DELETE' })
      if (!response?.ok) throw new Error('Could not clear MCP activity.')
      this.$mcpAudit = []
      window._caduceus?.toast('MCP activity cleared.')
    } catch (error) {
      this.$mcpError = error instanceof Error ? error.message : 'Could not clear MCP activity.'
    } finally {
      this.$mcpSaving = false
    }
  }

  async loadAliases() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const res = await apiFetch('/users/aliases')
      if (!res?.ok) return
      const data = await res.json()
      this.$primaryEmail = data?.email || ''
      this.$aliases = Array.isArray(data?.aliases) ? data.aliases : []
    } catch {
      this.$aliases = []
    }
  }

  async addAlias() {
    const alias = this.$newAlias.trim()
    if (!alias) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$aliasSaving = true
    this.$aliasError = ''
    try {
      const res = await apiFetch('/users/aliases', { method: 'POST', body: JSON.stringify({ alias }) })
      const data = await res?.json()
      if (!res?.ok) {
        this.$aliasError = data?.error || 'Could not add alias.'
        return
      }
      this.$primaryEmail = data?.email || this.$primaryEmail
      this.$aliases = Array.isArray(data?.aliases) ? data.aliases : this.$aliases
      this.$newAlias = ''
      window._caduceus?.toast('Alias added.')
    } catch {
      this.$aliasError = 'Network error.'
    } finally {
      this.$aliasSaving = false
    }
  }

  async removeAlias(alias) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const res = await apiFetch('/users/aliases', { method: 'DELETE', body: JSON.stringify({ alias }) })
    const data = await res?.json()
    if (!res?.ok) {
      this.$aliasError = data?.error || 'Could not remove alias.'
      return
    }
    this.$aliases = Array.isArray(data?.aliases) ? data.aliases : this.$aliases.filter(item => item !== alias)
    window._caduceus?.toast('Alias removed.')
  }

  /**
   * Persist a new calendar-visibility choice and reflect it locally.
   * @param {'private'|'busy'|'full'} v
   */
  async setVisibility(v) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$calendarVisibility = v
    try {
      const res = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ calendarVisibility: v }),
      })
      if (!res?.ok) { window._caduceus?.toast('Could not save preference.'); return }
      window._caduceus?.toast('Calendar sharing updated.')
    } catch {
      window._caduceus?.toast('Network error.')
    }
  }

  /** @param {Event} event */
  async setInterfaceFont(event) {
    const value = normalizeInterfaceFont(/** @type {HTMLSelectElement | null} */ (event?.target)?.value)
    await this.saveFontPreferences({ interfaceFont: value })
  }

  /** @param {Event} event */
  async setComposerFont(event) {
    const value = normalizeComposerFont(/** @type {HTMLSelectElement | null} */ (event?.target)?.value)
    await this.saveFontPreferences({ composerFont: value })
  }

  /**
   * Save one typography patch. The preview is optimistic, but a rejected or
   * failed request restores both previously persisted catalog identifiers.
   *
   * @param {{ interfaceFont?: string, composerFont?: string }} patch
   */
  async saveFontPreferences(patch) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$fontSaving) return
    const previous = { interfaceFont: this.$interfaceFont, composerFont: this.$composerFont }
    const optimistic = {
      interfaceFont: normalizeInterfaceFont(patch.interfaceFont ?? previous.interfaceFont),
      composerFont: normalizeComposerFont(patch.composerFont ?? previous.composerFont),
    }
    this.$interfaceFont = optimistic.interfaceFont
    this.$composerFont = optimistic.composerFont
    this.$fontSaving = true
    this.$fontError = ''
    window._caduceus?.setFontPreferences?.(optimistic)
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify(patch),
      })
      const data = await response?.json()
      if (!response?.ok) {
        this.$fontError = data?.error || 'Could not save typography preferences.'
        this.$interfaceFont = previous.interfaceFont
        this.$composerFont = previous.composerFont
        window._caduceus?.setFontPreferences?.(previous)
        return
      }
      this.$interfaceFont = normalizeInterfaceFont(data?.interfaceFont)
      this.$composerFont = normalizeComposerFont(data?.composerFont)
      window._caduceus?.setFontPreferences?.({
        interfaceFont: this.$interfaceFont,
        composerFont: this.$composerFont,
      })
      window._caduceus?.toast('Typography preference saved.')
    } catch {
      this.$fontError = 'Network error.'
      this.$interfaceFont = previous.interfaceFont
      this.$composerFont = previous.composerFont
      window._caduceus?.setFontPreferences?.(previous)
    } finally {
      this.$fontSaving = false
    }
  }

  /** @param {Event} event @param {unknown} soundEvent */
  async setNotificationSound(event, soundEvent) {
    if (!isSoundEventId(soundEvent)) return
    const soundId = /** @type {HTMLSelectElement | null} */ (event?.target)?.value
    if (!isSoundId(soundId)) return
    const previous = this.$notificationSounds
    const next = normalizeNotificationSounds({ ...previous, [soundEvent]: soundId })
    this.$notificationSounds = next
    window._caduceus?.setNotificationSounds?.(next)
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$soundSaving) return
    this.$soundSaving = true
    this.$soundError = ''
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ notificationSounds: next }),
      })
      const data = await response?.json()
      if (!response?.ok) {
        this.$soundError = data?.error || 'Could not save notification sounds.'
        this.$notificationSounds = previous
        window._caduceus?.setNotificationSounds?.(previous)
        return
      }
      this.$notificationSounds = normalizeNotificationSounds(data?.notificationSounds)
      window._caduceus?.setNotificationSounds?.(this.$notificationSounds)
      window._caduceus?.toast('Notification sound saved.')
    } catch {
      this.$soundError = 'Network error.'
      this.$notificationSounds = previous
      window._caduceus?.setNotificationSounds?.(previous)
    } finally {
      this.$soundSaving = false
    }
  }

  /** @param {Event} _event @param {unknown} soundEvent */
  async previewNotificationSound(_event, soundEvent) {
    if (!isSoundEventId(soundEvent)) return
    const played = await window._caduceus?.playSound?.(soundEvent, this.$notificationSounds[soundEvent])
    if (!played && this.$notificationSounds[soundEvent] !== 'none') {
      this.$soundError = 'This browser could not play the preview.'
    }
  }

  /** @param {Event} event */
  async setUnreadBadgeScope(event) {
    const scope = /** @type {HTMLSelectElement | null} */ (event?.target)?.value
    if (scope !== 'all' && scope !== 'domain') return
    await this.saveUnreadBadgePreference(scope, this.$unreadBadgeDomain || this.mailboxDomains[0] || '')
  }

  /** @param {Event} event */
  async setUnreadBadgeDomain(event) {
    const domain = /** @type {HTMLSelectElement | null} */ (event?.target)?.value || ''
    if (!this.mailboxDomains.includes(domain)) return
    await this.saveUnreadBadgePreference('domain', domain)
  }

  /** @param {'all'|'domain'} scope @param {string} domain */
  async saveUnreadBadgePreference(scope, domain) {
    if (this.$unreadBadgeSaving) return
    const previous = { scope: this.$unreadBadgeScope, domain: this.$unreadBadgeDomain }
    const next = normalizeUnreadBadgePreference({ scope, domain }, this.mailboxDomains)
    if (scope === 'domain' && next.scope !== 'domain') {
      this.$unreadBadgeError = 'Choose an account assigned to your profile.'
      return
    }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$unreadBadgeScope = next.scope
    this.$unreadBadgeDomain = next.domain
    this.$unreadBadgeSaving = true
    this.$unreadBadgeError = ''
    window._caduceus?.setUnreadBadgePreference?.(next)
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({
          unreadBadgeScope: next.scope,
          unreadBadgeDomain: next.domain,
        }),
      })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save unread badge preference.')
      const persisted = normalizeUnreadBadgePreference(data, this.mailboxDomains)
      this.$unreadBadgeScope = persisted.scope
      this.$unreadBadgeDomain = persisted.domain
      window._caduceus?.setUnreadBadgePreference?.(persisted)
      window._caduceus?.toast('Unread badge preference saved.')
    } catch (error) {
      this.$unreadBadgeScope = previous.scope
      this.$unreadBadgeDomain = previous.domain
      window._caduceus?.setUnreadBadgePreference?.(previous)
      this.$unreadBadgeError = error instanceof Error ? error.message : 'Could not save unread badge preference.'
    } finally {
      this.$unreadBadgeSaving = false
    }
  }

  /** @param {unknown} value */
  normalizeNewMailNotificationPolicy(value) {
    const input = value && typeof value === 'object'
      ? /** @type {Record<string, any>} */ (value)
      : {}
    const allowed = this.mailboxDomains
    const requested = Array.isArray(input.newMailNotificationDomains)
      ? input.newMailNotificationDomains
      : allowed
    const allowedExternal = this.$externalAccounts.map(account => account.id)
    const requestedExternal = Array.isArray(input.newMailNotificationExternalAccounts)
      ? input.newMailNotificationExternalAccounts
      : allowedExternal
    return {
      domains: [...new Set(requested.map(domain => String(domain).toLowerCase()).filter(domain => allowed.includes(domain)))],
      externalAccounts: [...new Set(requestedExternal.filter(id => allowedExternal.includes(id)))],
      privacy: ['full', 'sender', 'private'].includes(input.newMailNotificationPrivacy)
        ? input.newMailNotificationPrivacy
        : 'full',
      actions: typeof input.newMailNotificationActions === 'boolean'
        ? input.newMailNotificationActions
        : true,
    }
  }

  /** @param {Event} _event @param {string} domain */
  async toggleNewMailNotificationDomain(_event, domain) {
    const enabled = this.$newMailNotificationDomains.includes(domain)
    const domains = enabled
      ? this.$newMailNotificationDomains.filter(item => item !== domain)
      : [...this.$newMailNotificationDomains, domain]
    await this.saveNewMailNotificationPolicy({ newMailNotificationDomains: domains })
  }

  /** @param {Event} _event @param {string} accountId */
  async toggleNewMailNotificationExternalAccount(_event, accountId) {
    const enabled = this.$newMailNotificationExternalAccounts.includes(accountId)
    const accounts = enabled
      ? this.$newMailNotificationExternalAccounts.filter(item => item !== accountId)
      : [...this.$newMailNotificationExternalAccounts, accountId]
    await this.saveNewMailNotificationPolicy({ newMailNotificationExternalAccounts: accounts })
  }

  /** @param {Event} event */
  async setNewMailNotificationPrivacy(event) {
    const privacy = /** @type {HTMLSelectElement | null} */ (event.target)?.value || ''
    if (!['full', 'sender', 'private'].includes(privacy)) return
    await this.saveNewMailNotificationPolicy({ newMailNotificationPrivacy: privacy })
  }

  /** @param {Event} event */
  async setNewMailNotificationActions(event) {
    const actions = /** @type {HTMLInputElement | null} */ (event.target)?.checked
    if (typeof actions !== 'boolean') return
    await this.saveNewMailNotificationPolicy({ newMailNotificationActions: actions })
  }

  /** @param {Record<string, unknown>} patch */
  async saveNewMailNotificationPolicy(patch) {
    if (this.$newMailNotificationSaving) return
    const previous = {
      domains: this.$newMailNotificationDomains,
      externalAccounts: this.$newMailNotificationExternalAccounts,
      privacy: this.$newMailNotificationPrivacy,
      actions: this.$newMailNotificationActions,
    }
    const optimistic = this.normalizeNewMailNotificationPolicy({
      newMailNotificationDomains: previous.domains,
      newMailNotificationExternalAccounts: previous.externalAccounts,
      newMailNotificationPrivacy: previous.privacy,
      newMailNotificationActions: previous.actions,
      ...patch,
    })
    this.$newMailNotificationDomains = optimistic.domains
    this.$newMailNotificationExternalAccounts = optimistic.externalAccounts
    this.$newMailNotificationPrivacy = optimistic.privacy
    this.$newMailNotificationActions = optimistic.actions
    this.$newMailNotificationSaving = true
    this.$newMailNotificationError = ''
    try {
      const response = await window._caduceus?.apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify(patch),
      })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save new-mail notifications.')
      const persisted = this.normalizeNewMailNotificationPolicy(data)
      this.$newMailNotificationDomains = persisted.domains
      this.$newMailNotificationExternalAccounts = persisted.externalAccounts
      this.$newMailNotificationPrivacy = persisted.privacy
      this.$newMailNotificationActions = persisted.actions
      window._caduceus?.toast('New-mail notification preference saved.')
    } catch (error) {
      this.$newMailNotificationDomains = previous.domains
      this.$newMailNotificationExternalAccounts = previous.externalAccounts
      this.$newMailNotificationPrivacy = previous.privacy
      this.$newMailNotificationActions = previous.actions
      this.$newMailNotificationError = error instanceof Error ? error.message : 'Could not save new-mail notifications.'
    } finally {
      this.$newMailNotificationSaving = false
    }
  }

  /**
   * Assigned domains available for personal naming controls.
   *
   * @returns {string[]}
   */
  get mailboxDomains() {
    /** @type {string[]} */
    const domains = []
    if (Array.isArray(window._caduceus?.auth?.domains)) domains.push(...window._caduceus.auth.domains)
    try {
      const cached = JSON.parse(sessionStorage.getItem('caduceus_domains') || '[]')
      if (Array.isArray(cached)) domains.push(...cached)
    } catch {}
    return [...new Set(domains.map(domain => String(domain).toLowerCase()))].sort()
  }

  /**
   * @param {string} domain
   * @returns {string}
   */
  mailboxName(domain) { return this.$mailboxNames[String(domain).toLowerCase()] || '' }

  /**
   * @param {string} domain
   * @returns {string}
   */
  mailboxLabel(domain) { return this.mailboxName(domain) || String(domain).toLowerCase() }

  /**
   * Save or clear a display-only mailbox name. A blank value restores the
   * domain label while preserving mailbox access and delivery configuration.
   *
   * @param {string} domain
   * @param {string} name
   * @returns {Promise<void>}
   */
  async saveMailboxName(domain, name) {
    const normalizedDomain = String(domain || '').trim().toLowerCase()
    if (!this.mailboxDomains.includes(normalizedDomain)) {
      this.$mailboxError = 'Mailbox is not assigned to your account.'
      return
    }
    const next = { ...this.$mailboxNames }
    const normalizedName = String(name || '').trim()
    if (normalizedName) next[normalizedDomain] = normalizedName
    else delete next[normalizedDomain]

    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$mailboxSaving = true; this.$mailboxError = ''
    try {
      const res = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ mailboxNames: next }),
      })
      const data = await res?.json()
      if (!res?.ok) { this.$mailboxError = data?.error || 'Could not save mailbox name.'; return }
      this.$mailboxNames = data?.mailboxNames || {}
      window._caduceus?.setMailboxNames?.(this.$mailboxNames)
      window._caduceus?.toast(normalizedName ? 'Mailbox name saved.' : 'Mailbox name removed.')
    } catch {
      this.$mailboxError = 'Network error.'
    } finally {
      this.$mailboxSaving = false
    }
  }

  /**
   * @param {string} domain
   * @returns {Promise<void>}
   */
  async clearMailboxName(domain) { await this.saveMailboxName(domain, '') }

  /** Preserve native datalist edits before Tachyon can replace their nodes. */
  bindCategoryMappingDrafts() {
    const host = /** @type {any} */ (window)
    if (host._caduceusCategoryMappingInput) {
      document.removeEventListener('input', host._caduceusCategoryMappingInput, true)
    }
    host._caduceusCategoryMappingInput = event => {
      const input = event.target
      if (!(input instanceof HTMLInputElement) || !input.matches('.category-role-field input')) return
      const domain = input.dataset.categoryDomain || ''
      const role = input.dataset.categoryRole || ''
      if (domain && role) this._categoryMappingDrafts[`${domain}\u0000${role}`] = input.value
    }
    document.addEventListener('input', host._caduceusCategoryMappingInput, true)
  }

  /** Persist the flattened form in one complete, cross-domain transaction. */
  async saveAllCategoryMappings(event) {
    const trigger = event?.currentTarget instanceof Element ? event.currentTarget : null
    const section = trigger?.closest('[data-category-mapper]')
    const inputs = [...(section?.querySelectorAll('.category-role-field input') || [])]
    const rows = this.categoryRows
    if (inputs.length !== rows.length) {
      this.$categoryError = 'The folder-role form is incomplete.'
      return
    }
    const next = {}
    rows.forEach((row, index) => {
      next[row.domain] ||= {}
      const key = `${row.domain}\u0000${row.role}`
      next[row.domain][row.role] = Object.hasOwn(this._categoryMappingDrafts, key)
        ? this._categoryMappingDrafts[key]
        : /** @type {HTMLInputElement} */ (inputs[index]).value
    })
    const previous = this.$categoryMappings
    this.$categoryMappings = next
    await this.saveCategoryMappings(previous)
  }

  async resetAllCategoryMappings() {
    const previous = this.$categoryMappings
    this.$categoryMappings = Object.fromEntries(this.$categoryAccounts.map(account => [
      account.domain,
      Object.fromEntries(CATEGORY_ROLES.map(role => [role.id, role.id])),
    ]))
    await this.saveCategoryMappings(previous)
  }

  /** @param {Record<string, Record<string, string>>} previous */
  async saveCategoryMappings(previous) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$categorySaving) return
    this.$categorySaving = true
    this.$categoryError = ''
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ categoryMappings: this.$categoryMappings }),
      })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not save folder roles.')
      this.$categoryMappings = data?.categoryMappings || this.$categoryMappings
      this._categoryMappingDrafts = {}
      const catalogResponse = await apiFetch('/categories')
      const catalog = await catalogResponse?.json()
      if (catalogResponse?.ok) this.$categoryAccounts = Array.isArray(catalog?.accounts) ? catalog.accounts : []
      window._caduceus?.toast('Folder roles updated.')
    } catch (error) {
      this.$categoryMappings = previous
      this.$categoryError = error instanceof Error ? error.message : 'Could not save folder roles.'
    } finally {
      this.$categorySaving = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {'block'|'allow'} policy */
  async setRemoteImagePolicy(policy) {
    if (!['block', 'allow'].includes(policy)) return
    this.$remoteImagePolicy = policy
    await this.saveImagePrivacyPreferences({ remoteImagePolicy: policy })
  }

  /** @param {string} sender */
  async removeTrustedImageSender(sender) {
    const trustedImageSenders = this.$trustedImageSenders.filter(entry => entry !== sender)
    await this.saveImagePrivacyPreferences({ trustedImageSenders })
  }

  /** @param {Event} event */
  async setGitHubProfileEnrichment(event) {
    const target = event?.currentTarget instanceof HTMLInputElement
      ? event.currentTarget
      : document.querySelector('[data-github-profile-enrichment]')
    const enabled = target instanceof HTMLInputElement ? target.checked : false
    this.$githubProfileEnrichment = enabled
    await this.saveImagePrivacyPreferences({ githubProfileEnrichment: enabled })
  }

  /**
   * Persist one privacy patch and reconcile both values from the server.
   * @param {{ remoteImagePolicy?: 'block'|'allow', trustedImageSenders?: string[], githubProfileEnrichment?: boolean }} patch
   */
  async saveImagePrivacyPreferences(patch) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$privacySaving) return
    this.$privacySaving = true
    this.$privacyError = ''
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify(patch),
      })
      const data = await response?.json()
      if (!response?.ok) {
        this.$privacyError = data?.error || 'Could not save remote-content preferences.'
        return
      }
      this.$remoteImagePolicy = data?.remoteImagePolicy === 'allow' ? 'allow' : 'block'
      this.$trustedImageSenders = Array.isArray(data?.trustedImageSenders) ? data.trustedImageSenders : []
      this.$githubProfileEnrichment = data?.githubProfileEnrichment === true
      window._caduceus?.toast('Privacy preference saved.')
    } catch {
      this.$privacyError = 'Network error.'
    } finally {
      this.$privacySaving = false
    }
  }

  /** @type {boolean} Whether the Add User form is visible */
  $showAddUser = false; /** @type {boolean} Whether the Add Rule form is visible */
  $showAddRule = false
  /** @type {string} */
  $newEmail = ''; /** @type {string} */
  $newDomains = ''; /** @type {string} */
  $newRole = 'viewer'
  /** @type {string} */
  $newRuleName = ''; /** @type {string} */
  $newRuleMailbox = ''; /** @type {string} */
  $ruleEditId = ''; /** @type {boolean} */
  $newStopProcessing = false
  $newConditionMatch = 'all'
  /** @type {Array<Record<string, any>>} */
  $newConditions = []; /** @type {Array<Record<string, any>>} */
  $newActions = []
  /** @type {string} */
  $condField = 'from'; /** @type {string} */
  $condOp = 'contains'; /** @type {string} */
  $condValue = ''
  /** @type {string} */
  $actionType = 'folder'; /** @type {string} */
  $actionFolder = ''; /** @type {string} */
  $actionTo = ''
  /** @type {string} */
  $previewFrom = ''; /** @type {string} */
  $previewSubject = ''; /** @type {string} */
  $previewBody = ''; /** @type {boolean} */
  $previewHasAttachment = false; /** @type {string} */
  $rulePreviewResult = ''
  /**
   * Step-up state. Changing a credential needs a factor proven again, so the
   * view collects one before it calls any credential operation.
   * @type {boolean}
   */
  $showStepUp = false; /** @type {string} */
  $stepUpPassword = ''; /** @type {string} */
  $stepUpCode = ''; /** @type {string} */
  $stepUpError = ''
  /** @type {boolean} Whether the Add passkey wizard is open */
  $showAddDevice = false; /** @type {object|null} Challenge from the auth boundary */
  $deviceProvision = null; /** @type {string} */
  $newDeviceName = ''; /** @type {string} */
  $newDeviceCode = ''
  /** @type {boolean} */
  $deviceLoading = false; /** @type {string} */
  $deviceError = ''
  /** @type {object|null} */
  $totpEnrollment = null; /** @type {string} */
  $totpCode = ''; /** @type {string} */
  $totpName = 'Authenticator app'; /** @type {boolean} */
  $totpLoading = false; /** @type {string} */
  $totpError = ''
  /** @type {Record<string, { operatorCommand: string, note: string }>} */
  $adminRecoveryCodes = {}
  /** @type {string} */
  $newPassword = ''; /** @type {string} */
  $confirmPassword = ''; /** @type {boolean} */
  $passwordSaving = false; /** @type {string} */
  $passwordError = ''
  /** @type {string[]} Plaintext recovery codes are held only until dismissed. */
  $recoveryCodes = []; /** @type {boolean} */
  $recoveryLoading = false; /** @type {string} */
  $recoveryError = ''
  /** @type {boolean} Whether the browser supports Web Push */
  $notificationSupported = false; /** @type {string} Current Notification API permission state */
  $notificationPermission = 'default'
  /** @type {boolean} */
  $notificationLoading = false; /** @type {string} */
  $notificationError = ''
  /** @type {'all'|'domain'} Account set represented by app and push badges. */
  $unreadBadgeScope = 'all'
  /** @type {string} Assigned domain used when the unread badge is focused. */
  $unreadBadgeDomain = ''
  /** @type {boolean} */
  $unreadBadgeSaving = false
  /** @type {string} */
  $unreadBadgeError = ''
  /** @type {string[]} Hosted accounts allowed to produce new-mail push alerts. */
  $newMailNotificationDomains = []
  /** @type {string[]} Connected accounts allowed to produce new-mail push alerts. */
  $newMailNotificationExternalAccounts = []
  /** @type {'full'|'sender'|'private'} */
  $newMailNotificationPrivacy = 'full'
  /** @type {boolean} */
  $newMailNotificationActions = true
  /** @type {boolean} */
  $newMailNotificationSaving = false
  /** @type {string} */
  $newMailNotificationError = ''
  /** @type {Record<'new-mail'|'hit-send'|'send'|'confirm', 'none'|'chime'|'bell'|'pop'|'pulse'>} */
  $notificationSounds = normalizeNotificationSounds(DEFAULT_NOTIFICATION_SOUNDS)
  /** @type {boolean} */
  $soundSaving = false
  /** @type {string} */
  $soundError = ''
  /** @type {boolean} Whether the current user has the admin role */
  $isAdmin = false
  /** @type {Array<{ id: string, domain: string, name: string, text: string, defaultFor: string[], createdAt?: string, updatedAt?: string }>} */
  $signatures = []
  /** @type {string} */
  $signatureEditId = ''; /** @type {string} */
  $signatureName = ''; /** @type {string} */
  $signatureText = ''; /** @type {string[]} */
  $signatureDefaults = []; /** @type {string} */
  $signatureError = ''; /** @type {boolean} */
  $signatureSaving = false
  /** @type {Array<{ id: string, name: string, subject: string, text: string, to: string, cc: string, createdAt?: string, updatedAt?: string }>} */
  $templates = []
  /** @type {string} */
  $templateEditId = ''; /** @type {string} */
  $templateName = ''; /** @type {string} */
  $templateSubject = ''; /** @type {string} */
  $templateTo = ''; /** @type {string} */
  $templateCc = ''; /** @type {string} */
  $templateText = ''; /** @type {string} */
  $templateError = ''; /** @type {boolean} */
  $templateSaving = false

  /** @type {number} Internal sequence counter for condition IDs */
  _condSeq = 0; /** @type {number} Internal sequence counter for action IDs */
  _actSeq = 0
  /** @type {string} Key used to persist MFA setup state on the window object */
  _mfaSetupStateKey = '__caduceus_mfa_setup_state'

  /**
   * @description Whether the current browser supports the Web Push and
   * Service Worker APIs required for push notifications.
   * @returns {boolean}
   */
  get canUseNotifications() { return !window.__CADUCEUS_DISABLE_SW && 'Notification' in window && 'serviceWorker' in navigator && 'PushManager' in window }

  /**
   * @description Human-readable label summarising the current notification status.
   * @returns {string}
   */
  get notificationStatusLabel() {
    if (!this.$notificationSupported) return 'Not available on this browser'
    if (this.$notificationPermission === 'granted' && this.$pushSubscriptions.length > 0) return 'On'
    if (this.$notificationPermission === 'denied') return 'Blocked'
    return 'Off'
  }

  /**
   * @description Contextual help text explaining the current notification state
   * and what the user can do to enable or manage alerts.
   * @returns {string}
   */
  get notificationHelpText() {
    if (!this.$notificationSupported) return 'Install Caduceus with a browser that supports Web Push to receive new mail alerts.'
    if (this.$notificationPermission === 'denied') return 'Notifications are blocked in this browser. Allow them in site settings to turn alerts on.'
    if (this.$notificationPermission === 'granted' && this.$pushSubscriptions.length > 0) return 'This device will alert you when new mail arrives.'
    return 'Turn this on for an installed desktop or mobile app feel when mail arrives.'
  }

  get soundEvents() { return SOUND_EVENTS }
  get soundOptions() { return SOUND_OPTIONS }
  get unreadBadgeDomainOptions() {
    return this.mailboxDomains.map(domain => ({ domain, label: this.mailboxLabel(domain) }))
  }
  get newMailNotificationDomainOptions() {
    return this.mailboxDomains.map(domain => ({
      domain,
      label: this.mailboxLabel(domain),
      enabled: this.$newMailNotificationDomains.includes(domain),
    }))
  }
  get newMailNotificationExternalAccountOptions() {
    return this.$externalAccounts.map(account => ({
      id: account.id,
      label: account.label || account.mailboxAddress,
      address: account.mailboxAddress,
      enabled: this.$newMailNotificationExternalAccounts.includes(account.id),
    }))
  }

  /** @param {unknown} event */
  notificationSound(event) {
    return isSoundEventId(event) ? this.$notificationSounds[event] : 'none'
  }

  /** Keep native select values aligned after Tachyon replaces their options. */
  syncSoundControls() {
    for (const { id } of SOUND_EVENTS) {
      const select = /** @type {HTMLSelectElement | null} */ (document.querySelector(`[data-sound-event="${id}"] select`))
      const value = this.notificationSound(id)
      if (select && select.value !== value) select.value = value
    }
  }

  get externalAccountRows() {
    return this.$externalAccounts.map(account => ({
      ...account,
      ...accountActions(account),
      progressLabel: accountProgressLabel(account),
      stateLabel: String(account.state || '').replaceAll('_', ' '),
      reconnectUsername: account.connection?.imap?.username || account.mailboxAddress,
      isMicrosoft: account.provider === 'microsoft',
      isOAuth: account.provider === 'microsoft' || account.provider === 'gmail',
      isSharedMailbox: account.provider === 'microsoft' && Boolean(account.sharedMailboxAddress),
      delegateIdentity: account.authIdentity && account.authIdentity !== account.mailboxAddress ? account.authIdentity : '',
      canRead: account.capabilities?.mailRead === true,
      canSend: account.capabilities?.send === true,
      reconnecting: this.$accountReconnectId === account.id,
      confirmingRemoval: this.$accountRemoveId === account.id,
      showingDiagnostics: this.$accountDiagnosticsId === account.id,
      diagnostics: this.$accountDiagnostics[account.id] || null,
      mutating: this.$accountMutationId === account.id,
    }))
  }

  get selectedProvider() {
    return this.$providerCatalog.find(provider => provider.id === this.$accountConnectionDraft.choiceId) || null
  }

  get selectedProviderGuidance() {
    if (this.selectedProvider?.guidance === 'app_password') return 'This provider requires an app-specific password.'
    if (this.selectedProvider?.guidance === 'enable_imap') return 'Enable IMAP access in the provider before connecting.'
    return ''
  }

  /** Load only redacted account projections and the public provider catalog. */
  async loadExternalAccounts() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$externalAccountsError = ''
    try {
      const [accountsResponse, providersResponse] = await Promise.all([
        apiFetch('/api/v1/external-accounts'), apiFetch('/api/v1/provider-catalog'),
      ])
      const accountsBody = await accountsResponse?.json()
      const providersBody = await providersResponse?.json()
      if (!accountsResponse?.ok) throw new Error(accountsBody?.error || 'Could not load connected accounts.')
      if (!providersResponse?.ok) throw new Error(providersBody?.error || 'Could not load providers.')
      this.$externalAccounts = normalizeExternalAccounts(accountsBody?.accounts)
      this.$providerCatalog = normalizeProviderCatalog(providersBody?.providers)
    } catch (error) {
      this.$externalAccountsError = error instanceof Error ? error.message : 'Could not load connected accounts.'
    }
  }

  openAccountSetup() {
    this.$accountSetupOpen = true
    this.$accountSetupStep = 'address'
    this.$accountSetupError = ''
    this.$accountSetupStatus = ''
    this.$accountAddress = ''
    this.$accountConnectionDraft = connectionDraft({}, '')
  }

  startAccountOnboarding() {
	this.$accountOnboarding = true
	try { localStorage.setItem('caduceus_account_onboarding_pending', '1') } catch {}
	this.openAccountSetup()
  }

  dismissAccountOnboarding() {
	this.$accountOnboarding = false
	try {
		localStorage.removeItem('caduceus_account_onboarding_pending')
		localStorage.setItem(`caduceus_account_onboarding_complete:${sessionStorage.getItem('caduceus_email') || 'owner'}`, '1')
	} catch {}
	this.closeAccountSetup()
  }

  closeAccountSetupOrOnboarding() {
	if (this.$accountOnboarding) this.dismissAccountOnboarding()
	else this.closeAccountSetup()
  }

  closeAccountSetup() {
    this.$accountSetupOpen = false
    this.$accountSetupBusy = false
    this.$accountSetupError = ''
    this.$accountSetupStatus = ''
    this.$accountConnectionDraft = connectionDraft({}, '')
  }

  /** @param {Event} event */
  updateAccountAddress(event) {
    const target = event.target instanceof HTMLInputElement ? event.target : null
    if (target) this.$accountAddress = target.value
  }

  /** @param {Event | null} _event @param {Record<string, any>} provider */
  async selectAccountProvider(_event, provider) {
    if (!provider) return
    if (provider.authentication === 'oauth') {
      if (!provider.configured) {
        this.$accountSetupError = `${provider.name} OAuth must be configured by this server’s administrator before this account can be connected.`
        await (/** @type {any} */ (globalThis)).__tc_rerender?.()
        return
      }
      const address = this.$accountAddress.trim().toLowerCase()
      this.$accountSetupError = ''
      this.$accountSetupStatus = ''
      this.$accountSetupStep = 'oauth'
      this.$accountConnectionDraft = { ...connectionDraft({}, address), choiceId: provider.id, oauthProvider: provider.provider, authIdentity: address, sharedMailboxAddress: '' }
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      return
    }
    this.$accountSetupError = ''
    this.$accountSetupStep = 'credentials'
    this.$accountConnectionDraft = connectionDraft({
      choiceId: provider.id, source: provider.preset ? 'preset' : 'manual', connection: provider.preset || {},
    }, this.$accountAddress)
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  async discoverExternalAccount() {
    const apiFetch = window._caduceus?.apiFetch
    const addressInput = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-external-account-address]'))
    const address = (addressInput?.value || this.$accountAddress).trim().toLowerCase()
    if (!apiFetch || this.$accountSetupBusy) return
    if (!address || !address.includes('@')) {
      this.$accountSetupError = 'Enter a complete email address.'
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      return
    }
    this.$accountAddress = address
    this.$accountSetupBusy = true
    this.$accountSetupError = ''
    this.$accountSetupStatus = 'Discovering secure mail settings…'
    try {
      const response = await apiFetch('/api/v1/provider-discovery', {
        method: 'POST', body: JSON.stringify({ address }),
      })
      const body = await response?.json()
      if (!response?.ok) throw new Error(body?.error || 'Mail settings could not be discovered.')
      const provider = this.$providerCatalog.find(choice => choice.id === body?.choiceId)
      if (provider?.authentication === 'oauth') {
        await this.selectAccountProvider(null, provider)
        if (this.$accountSetupStep !== 'oauth') throw new Error(this.$accountSetupError || `${provider.name} OAuth is not available.`)
        this.$accountSetupStatus = `${provider.name} settings found. Confirm the sign-in address to continue.`
        return
      }
      this.$accountConnectionDraft = connectionDraft(body, address)
      this.$accountSetupStep = 'credentials'
      this.$accountSetupStatus = body?.source === 'fallback'
        ? 'Suggested settings found. Review every server field before connecting.'
        : 'Mail settings discovered. Add your credentials to continue.'
    } catch (error) {
      this.$accountSetupError = error instanceof Error ? error.message : 'Mail settings could not be discovered.'
      this.$accountSetupStatus = ''
    } finally {
      this.$accountSetupBusy = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /**
   * Start provider authorization for a new or existing account.
   * @param {Event | null} _event
   * @param {Record<string, any> | null} reconnectAccount
   */
  async beginProviderOAuth(_event, reconnectAccount = null) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$accountSetupBusy || this.$accountMutationId) return
    const fieldValue = (field) => /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-oauth-account-field="${field}"]`))?.value?.trim() || ''
    const provider = reconnectAccount?.provider || this.selectedProvider?.provider || this.$accountConnectionDraft.oauthProvider
    const authIdentity = reconnectAccount ? reconnectAccount.authIdentity : fieldValue('authIdentity').toLowerCase()
    const sharedMailboxAddress = reconnectAccount ? reconnectAccount.sharedMailboxAddress : fieldValue('sharedMailboxAddress').toLowerCase()
    const mailboxAddress = reconnectAccount ? reconnectAccount.mailboxAddress : (sharedMailboxAddress || authIdentity)
    if (!['microsoft', 'gmail'].includes(provider)) return
    if (!reconnectAccount && (!authIdentity || !authIdentity.includes('@') || (sharedMailboxAddress && !sharedMailboxAddress.includes('@')))) {
      this.$accountSetupError = 'Enter a complete provider sign-in address and, when used, a complete shared mailbox address.'
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      return
    }
    const request = reconnectAccount ? {
      provider, accountId: reconnectAccount.id, revision: reconnectAccount.revision,
    } : {
      provider, authIdentity, mailboxAddress, sharedMailboxAddress,
      label: fieldValue('label') || mailboxAddress, color: fieldValue('color') || (provider === 'gmail' ? '#EA4335' : '#0078D4'), order: this.$externalAccounts.length,
    }
    this.$accountSetupBusy = !reconnectAccount
    this.$accountMutationId = reconnectAccount?.id || ''
    this.$accountSetupError = ''
    this.$externalAccountsError = ''
    this.$accountSetupStatus = `Preparing a secure ${provider === 'gmail' ? 'Google' : 'Microsoft'} sign-in…`
    try {
      const response = await apiFetch(`/api/v1/external-accounts/oauth/${provider === 'gmail' ? 'google' : 'microsoft'}/begin`, { method: 'POST', body: JSON.stringify(request) })
      const body = await response?.json()
      if (!response?.ok) throw new Error(body?.error || 'Provider authorization could not be started.')
      const authorization = new URL(String(body?.authorizationUrl || ''))
      const expectedHost = provider === 'gmail' ? 'accounts.google.com' : 'login.microsoftonline.com'
      if (authorization.protocol !== 'https:' || authorization.hostname !== expectedHost) throw new Error('The server returned an invalid provider authorization address.')
      sessionStorage.setItem('caduceus_oauth_provider', provider)
      try { localStorage.setItem(`caduceus_oauth_provider:${body.state}`, provider) } catch {}
      if (this.$accountOnboarding) {
        try { localStorage.setItem('caduceus_account_onboarding_pending', '1') } catch {}
      }
      window.location.assign(authorization.href)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Provider authorization could not be started.'
      if (reconnectAccount) this.$externalAccountsError = message
      else this.$accountSetupError = message
      this.$accountSetupStatus = ''
      this.$accountSetupBusy = false
      this.$accountMutationId = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** Consume a same-origin OAuth redirect once and remove secrets from browser history. */
  async completeProviderOAuthCallback() {
    const url = new URL(window.location.href)
    const hasOAuthParameters = ['code', 'state', 'error'].some(name => url.searchParams.has(name))
    if (!hasOAuthParameters) return
    const code = url.searchParams.get('code') || ''
    const state = url.searchParams.get('state') || ''
    const providerError = url.searchParams.has('error')
    let stateProvider = ''
    try {
      stateProvider = localStorage.getItem(`caduceus_oauth_provider:${state}`) || ''
      localStorage.removeItem(`caduceus_oauth_provider:${state}`)
    } catch {}
    const sessionProvider = sessionStorage.getItem('caduceus_oauth_provider') || ''
    sessionStorage.removeItem('caduceus_oauth_provider')
    const storedProvider = ['gmail', 'microsoft'].includes(stateProvider) ? stateProvider : sessionProvider
    for (const name of ['code', 'state', 'error', 'error_description', 'error_uri', 'session_state']) url.searchParams.delete(name)
    window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`)
    if (providerError || !code || !state) {
      this.$externalOAuthNotice = 'Provider authorization was not completed. No account credentials were changed.'
      return
    }
    try {
      const candidates = storedProvider === 'gmail' || storedProvider === 'microsoft' ? [storedProvider] : ['microsoft', 'gmail']
      /** @type {Record<string, any> | null} */
      let body = null
      for (const provider of candidates) {
        const response = await window._caduceus?.apiFetch?.(`/api/v1/external-accounts/oauth/${provider === 'gmail' ? 'google' : 'microsoft'}/complete`, {
          method: 'POST', body: JSON.stringify({ code, state }),
        })
        const candidate = await response?.json()
        if (response?.ok) { body = candidate; break }
      }
      if (!body) throw new Error('Provider authorization could not be completed.')
      this.$externalOAuthNotice = `${body?.sharedMailboxAddress ? 'Shared mailbox' : body?.provider === 'gmail' ? 'Google account' : 'Microsoft account'} connected successfully.`
      this.completeAccountOnboarding()
      window._caduceus?.toast(this.$externalOAuthNotice)
      window.dispatchEvent(new CustomEvent('caduceus:external-accounts-changed'))
    } catch {
      this.$externalOAuthNotice = 'Provider authorization could not be completed. Try reconnecting the account.'
    }
  }

  completeAccountOnboarding() {
	if (!this.$accountOnboarding) return
	this.$accountOnboarding = false
	this.$accountSetupOpen = false
	try {
		localStorage.removeItem('caduceus_account_onboarding_pending')
		localStorage.setItem(`caduceus_account_onboarding_complete:${sessionStorage.getItem('caduceus_email') || 'owner'}`, '1')
	} catch {}
  }

  /** @param {Event} event */
  updateAccountConnectionField(event) {
    const target = event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement ? event.target : null
    const field = target?.dataset.externalAccountField || ''
    if (!target || !field || field === 'password') return
    const passwordValue = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-external-account-field="password"]'))?.value || ''
    this.$accountConnectionDraft = { ...this.$accountConnectionDraft, [field]: target.value }
    // Tachyon replaces the native fieldset after a reactive edit. Keep the
    // secret only in this one-tick closure so a later server-field edit does
    // not silently erase it; it never enters component state or storage.
    if (passwordValue) setTimeout(() => {
      const password = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-external-account-field="password"]'))
      if (password && !password.value) password.value = passwordValue
    }, 0)
  }

  /** Build one request from native controls. The password exists only in this stack frame. */
  accountRequestFromControls() {
    const value = (field) => /** @type {HTMLInputElement | HTMLSelectElement | null} */ (document.querySelector(`[data-external-account-field="${field}"]`))?.value?.trim() || ''
    const mailboxAddress = this.$accountConnectionDraft.mailboxAddress || this.$accountAddress.trim().toLowerCase()
    const username = value('username')
    const password = value('password')
    const imapPort = Number(value('imapPort'))
    const smtpPort = Number(value('smtpPort'))
    if (!mailboxAddress || !username || !password) throw new Error('Email, username, and password are required.')
    if (!value('imapHost') || !value('smtpHost') || !Number.isInteger(imapPort) || !Number.isInteger(smtpPort)) {
      throw new Error('Complete both IMAP and SMTP server settings.')
    }
    const cardDAVURL = value('cardDAVURL')
    const calDAVURL = value('calDAVURL')
    return {
      provider: 'imap', mailboxAddress,
      label: value('label') || mailboxAddress, color: value('color') || '#5f6368', order: this.$externalAccounts.length,
      containerFolder: value('containerFolder'), credentials: { username, password },
      imap: { host: value('imapHost'), port: imapPort, security: value('imapSecurity'), username, allowInsecureTLS: false },
      smtp: { host: value('smtpHost'), port: smtpPort, security: value('smtpSecurity'), username, allowInsecureTLS: false },
      ...(cardDAVURL ? { carddav: { url: cardDAVURL, username: value('cardDAVUsername') || username, allowInsecureTLS: false } } : {}),
      ...(calDAVURL ? { caldav: { url: calDAVURL, username: value('calDAVUsername') || username, allowInsecureTLS: false } } : {}),
    }
  }

  async validateAndCreateExternalAccount() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$accountSetupBusy) return
    /** @type {Record<string, any>} */
    let request
    try { request = this.accountRequestFromControls() } catch (error) {
      this.$accountSetupError = error instanceof Error ? error.message : 'Complete the account form.'
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      return
    }
    this.$accountSetupBusy = true
    this.$accountSetupError = ''
    const davServices = [request.carddav && 'CardDAV', request.caldav && 'CalDAV'].filter(Boolean)
    this.$accountSetupStatus = davServices.length
      ? `Validating IMAP, SMTP, and ${davServices.join(' and ')} without sending mail…`
      : 'Validating IMAP and SMTP without sending mail…'
    const fieldset = /** @type {HTMLFieldSetElement | null} */ (document.querySelector('[data-external-account-fieldset]'))
    if (fieldset) fieldset.disabled = true
    try {
      const validationResponse = await apiFetch('/api/v1/external-accounts/validate', {
        method: 'POST', body: JSON.stringify(request),
      })
      const validationBody = await validationResponse?.json()
      if (!validationResponse?.ok) throw new Error(validationBody?.error || 'The mail servers rejected these settings.')
      this.$accountSetupStatus = 'Validation passed. Saving the encrypted account credential…'
      const createResponse = await apiFetch('/api/v1/external-accounts', {
        method: 'POST', body: JSON.stringify(request),
      })
      const createBody = await createResponse?.json()
      if (!createResponse?.ok) throw new Error(createBody?.error || 'The account could not be added.')
      this.$externalAccounts = normalizeExternalAccounts([...this.$externalAccounts, createBody])
	  this.completeAccountOnboarding()
      this.closeAccountSetup()
      window._caduceus?.toast('External account connected.')
    } catch (error) {
      this.$accountSetupError = error instanceof Error ? error.message : 'The account could not be added.'
      this.$accountSetupStatus = ''
      const password = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-external-account-field="password"]'))
      if (password) password.value = ''
    } finally {
      request.credentials.password = ''
      this.$accountSetupBusy = false
      if (fieldset) fieldset.disabled = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Event} _event @param {Record<string, any>} account @param {'pause'|'resume'} action */
  async mutateExternalAccount(_event, account, action) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$accountMutationId) return
    this.$accountMutationId = account.id
    this.$externalAccountsError = ''
    try {
      const response = await apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}`, {
        method: 'PATCH', body: JSON.stringify({ action, revision: account.revision }),
      })
      const body = await response?.json()
      if (!response?.ok) throw new Error(body?.error || `Could not ${action} this account.`)
      this.replaceExternalAccount(body)
    } catch (error) {
      this.$externalAccountsError = error instanceof Error ? error.message : `Could not ${action} this account.`
      await this.loadExternalAccounts()
    } finally {
      this.$accountMutationId = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Event} _event @param {Record<string, any>} account */
  async retryExternalAccount(_event, account) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$accountMutationId) return
    this.$accountMutationId = account.id
    try {
      const response = await apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}/retry`, {
        method: 'POST', body: JSON.stringify({ revision: account.revision }),
      })
      const body = await response?.json()
      if (!response?.ok || body?.scheduled !== true) throw new Error(body?.error || 'Could not schedule an account retry.')
      window._caduceus?.toast('Account retry scheduled.')
      await this.loadAccountDiagnostics(null, account)
    } catch (error) {
      this.$externalAccountsError = error instanceof Error ? error.message : 'Could not schedule an account retry.'
    } finally {
      this.$accountMutationId = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Event | null} _event @param {Record<string, any>} account */
  async loadAccountDiagnostics(_event, account) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    if (_event && this.$accountDiagnosticsId === account.id) {
      this.$accountDiagnosticsId = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      return
    }
    this.$accountDiagnosticsId = account.id
    try {
      const response = await apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}/diagnostics`)
      const body = await response?.json()
      if (!response?.ok) throw new Error(body?.error || 'Could not load account diagnostics.')
      const sync = body?.sync && typeof body.sync === 'object' ? body.sync : {}
      this.$accountDiagnostics = { ...this.$accountDiagnostics, [account.id]: {
        cursorVersion: Number(sync.cursorVersion) || 0,
        attempts: Number(sync.attempts) || 0,
        nextRunAt: dateTimeLabel(sync.nextRunAt) || 'Not scheduled',
        lastCode: typeof sync.lastCode === 'string' ? sync.lastCode : '',
        updatedAt: dateTimeLabel(sync.scheduleUpdatedAt) || '',
      } }
    } catch (error) {
      this.$externalAccountsError = error instanceof Error ? error.message : 'Could not load account diagnostics.'
    }
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  /** @param {Event} _event @param {Record<string, any>} account */
  async openAccountReconnect(_event, account) {
    this.$accountReconnectId = account.id
    this.$accountRemoveId = ''
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  cancelAccountReconnect() { this.$accountReconnectId = '' }

  /** @param {Event} _event @param {Record<string, any>} account */
  async reconnectExternalAccount(_event, account) {
    if (account.provider === 'microsoft' || account.provider === 'gmail') {
      await this.beginProviderOAuth(null, account)
      return
    }
    const apiFetch = window._caduceus?.apiFetch
    const username = /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-reconnect-username="${account.id}"]`))
    const password = /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-reconnect-password="${account.id}"]`))
    if (!apiFetch || this.$accountMutationId || !password?.value) return
    this.$accountMutationId = account.id
    this.$externalAccountsError = ''
    try {
      const response = await apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}/reconnect`, {
        method: 'POST', body: JSON.stringify({
          revision: account.revision,
          credentials: { username: username?.value?.trim() || account.mailboxAddress, password: password.value },
        }),
      })
      password.value = ''
      const body = await response?.json()
      if (!response?.ok) throw new Error(body?.error || 'Could not reconnect this account.')
      this.replaceExternalAccount(body)
      this.$accountReconnectId = ''
      window._caduceus?.toast('Account reconnected.')
    } catch (error) {
      if (password) password.value = ''
      this.$externalAccountsError = error instanceof Error ? error.message : 'Could not reconnect this account.'
      await this.loadExternalAccounts()
    } finally {
      this.$accountMutationId = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Event} _event @param {Record<string, any>} account */
  async askRemoveExternalAccount(_event, account) {
    this.$accountRemoveId = account.id
    this.$accountReconnectId = ''
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  cancelRemoveExternalAccount() { this.$accountRemoveId = '' }

  /** @param {Event} _event @param {Record<string, any>} account */
  async removeExternalAccount(_event, account) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$accountMutationId) return
    this.$accountMutationId = account.id
    try {
      const response = await apiFetch(`/api/v1/external-accounts/${encodeURIComponent(account.id)}`, {
        method: 'DELETE', body: JSON.stringify({ revision: account.revision }),
      })
      const body = await response?.json()
      if (!response?.ok) throw new Error(body?.error || 'Could not remove this account.')
      this.$externalAccounts = this.$externalAccounts.map(candidate => candidate.id === account.id
        ? { ...candidate, state: 'deleting', revision: candidate.revision + 1, progress: null, failure: null }
        : candidate)
      this.$accountRemoveId = ''
      window._caduceus?.toast('Account removal started.')
    } catch (error) {
      this.$externalAccountsError = error instanceof Error ? error.message : 'Could not remove this account.'
      await this.loadExternalAccounts()
    } finally {
      this.$accountMutationId = ''
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Record<string, any>} value */
  replaceExternalAccount(value) {
    const [account] = normalizeExternalAccounts([value])
    if (!account) return
    this.$externalAccounts = normalizeExternalAccounts(this.$externalAccounts.map(candidate => candidate.id === account.id ? account : candidate))
  }

  /**
   * Load all settings data in parallel on mount.
   *
   * Fetches domains, rules, MFA devices, notifications subscriptions, and
   * signatures concurrently. If the user is an admin, also fetches the user list.
   * Restores any in-progress MFA setup wizard state.
   *
   * @async
   * @returns {Promise<void>}
   */
  async loadAll() {
    this.$loading = true; this.syncNotificationState(); this.loadLocalTranslationPreferences()
    this.$isAdmin = sessionStorage.getItem('caduceus_role') === 'admin'
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) { this.$loading = false; return }
	const onboardingURL = new URL(window.location.href)
	let pendingOnboarding = onboardingURL.searchParams.get('onboarding') === 'accounts'
	try { pendingOnboarding ||= localStorage.getItem('caduceus_account_onboarding_pending') === '1' } catch {}
	if (onboardingURL.searchParams.has('onboarding')) {
		onboardingURL.searchParams.delete('onboarding')
		window.history.replaceState(window.history.state, '', `${onboardingURL.pathname}${onboardingURL.search}${onboardingURL.hash}`)
	}
	if (pendingOnboarding) this.startAccountOnboarding()
    await this.completeProviderOAuthCallback()
    const [dr, rr, mr, nr, sr, tr, cr] = await Promise.all([apiFetch('/domains'), apiFetch('/rules'), apiFetch('/auth/credentials/passkey'), apiFetch('/notifications/subscriptions'), apiFetch('/signatures'), apiFetch('/templates'), apiFetch('/categories')])
    this.$domains = dr?.ok ? await dr.json() : []; this.$rules = rr?.ok ? await rr.json() : []
    this.$mfaDevices = mr?.ok ? ((await mr.json())?.passkeys ?? []) : []; this.$signatures = sr?.ok ? await sr.json() : []; this.$pushSubscriptions = nr?.ok ? await nr.json() : []; this.$templates = tr?.ok ? await tr.json() : []
    if (cr?.ok) {
      const catalog = await cr.json()
      this.$categoryAccounts = Array.isArray(catalog?.accounts) ? catalog.accounts : []
      this.$categoryMappings = Object.fromEntries(this.$categoryAccounts.map(account => [account.domain, account.mappings]))
    }
    if (!('__caduceusTemplateEditId' in window)) (/** @type {any} */ (window)).__caduceusTemplateEditId = ''
    if (!('__caduceusSignatureEditId' in window)) (/** @type {any} */ (window)).__caduceusSignatureEditId = ''
    if (!('__caduceusRuleEditId' in window)) (/** @type {any} */ (window)).__caduceusRuleEditId = ''
    if (this.$isAdmin) { const ur = await apiFetch('/users'); this.$users = ur?.ok ? await ur.json() : [] }
    await Promise.all([this.loadAliases(), this.loadMcp(), this.loadExternalAccounts(), this.loadExtensions()])
    await this.loadPreferences()
    this.restoreMfaSetupState(); this.$loading = false
    setTimeout(async () => {
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      this.syncSoundControls()
      this.syncThemeControls()
      if (location.hash === '#keyboard-shortcuts') document.querySelector('[data-keyboard-shortcut-settings]')?.scrollIntoView({ block: 'start' })
    }, 0)
  }

  /** Load redacted extension projections; executable source stays off this endpoint. */
  async loadExtensions() {
    const response = await window._caduceus?.apiFetch?.('/extensions')
    this.$extensions = response?.ok ? await response.json() : []
  }

  /** Inspect a local JSON package without granting it authority or storing it. */
  async inspectExtensionFile(event) {
    const input = /** @type {HTMLInputElement} */ (event?.target)
    const file = input?.files?.[0]
    this.$extensionError = ''
    this.$extensionInspection = null
    this._extensionBundle = null
    if (!file) return
    if (file.size > 192 * 1024) { this.$extensionError = 'Extension package exceeds 192 KiB.'; return }
    this.$extensionBusy = true
    try {
      const bundle = JSON.parse(await file.text())
      const response = await window._caduceus?.apiFetch?.('/extensions/inspect', {
        method: 'POST', body: JSON.stringify({ bundle }),
      })
      const data = await response?.json()
      if (!response?.ok) { this.$extensionError = data?.error || 'Extension inspection failed.'; return }
      this._extensionBundle = bundle
      this.$extensionInspection = data
    } catch (error) {
      this.$extensionError = error instanceof SyntaxError ? 'Extension package is not valid JSON.' : 'Extension inspection failed.'
    } finally { this.$extensionBusy = false }
  }

  /** Install only the exact immutable digest the owner just inspected. */
  async trustExtension() {
    if (!this._extensionBundle || !this.$extensionInspection?.digest) return
    this.$extensionBusy = true
    this.$extensionError = ''
    try {
      const response = await window._caduceus?.apiFetch?.('/extensions', {
        method: 'POST',
        body: JSON.stringify({ bundle: this._extensionBundle, trustedDigest: this.$extensionInspection.digest }),
      })
      const data = await response?.json()
      if (!response?.ok) { this.$extensionError = data?.error || 'Extension installation failed.'; return }
      this.$extensionInspection = null
      this._extensionBundle = null
      await this.loadExtensions()
      window._caduceus?.toast(`“${data.displayName}” installed disabled.`)
    } catch { this.$extensionError = 'Extension installation failed.' }
    finally { this.$extensionBusy = false }
  }

  /** Enable/disable a package, then rebuild the owner runtime from trusted state. */
  async toggleExtension(event) {
    const id = String(/** @type {HTMLElement | null} */ (event?.currentTarget)?.closest?.('[data-extension-id]')?.getAttribute('data-extension-id') || '')
    const extension = this.$extensions.find(candidate => candidate.id === id)
    if (!extension) { this.$extensionError = 'Extension state changed; reload settings and try again.'; return }
    this.$extensionBusy = true
    this.$extensionError = ''
    try {
      const response = await window._caduceus?.apiFetch?.(`/extensions/${encodeURIComponent(extension.id)}`, {
        method: 'PUT', body: JSON.stringify({ enabled: !extension.enabled }),
      })
      const data = await response?.json()
      if (!response?.ok) { this.$extensionError = data?.error || 'Extension update failed.'; return }
      await this.loadExtensions()
      window.dispatchEvent(new CustomEvent('caduceus:extensions-reload'))
    } catch { this.$extensionError = 'Extension update failed.' }
    finally { this.$extensionBusy = false }
  }

  /** Permanently remove one exact-owner package. */
  async removeExtension(event) {
    const id = String(/** @type {HTMLElement | null} */ (event?.currentTarget)?.closest?.('[data-extension-id]')?.getAttribute('data-extension-id') || '')
    const extension = this.$extensions.find(candidate => candidate.id === id)
    if (!extension) { this.$extensionError = 'Extension state changed; reload settings and try again.'; return }
    if (!confirm(`Remove “${extension.displayName}” and its trusted code?`)) return
    this.$extensionBusy = true
    this.$extensionError = ''
    try {
      const response = await window._caduceus?.apiFetch?.(`/extensions/${encodeURIComponent(extension.id)}`, { method: 'DELETE' })
      const data = await response?.json()
      if (!response?.ok) { this.$extensionError = data?.error || 'Extension removal failed.'; return }
      await this.loadExtensions()
      window.dispatchEvent(new CustomEvent('caduceus:extensions-reload'))
    } catch { this.$extensionError = 'Extension removal failed.' }
    finally { this.$extensionBusy = false }
  }

  extensionPermissionLabel(permission) {
    return ({
      'compose:read': 'Read the current draft',
      'compose:write': 'Change the current draft',
      'mail:metadata': 'Read open-message metadata',
      'ui:notify': 'Show a local notification',
    })[permission] || permission
  }

  loadLocalTranslationPreferences() {
    const preferences = loadTranslationPreferences()
    this.$translationAutomaticLanguages = preferences.automatic
    this.$translationDisabledLanguages = preferences.disabled
  }

  /** @param {Event} _event @param {string} language */
  async restoreTranslationOffer(_event, language) {
    setTranslationPreference(language, 'offer')
    this.loadLocalTranslationPreferences()
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  async resetAllTranslationPreferences() {
    resetTranslationPreferences()
    this.loadLocalTranslationPreferences()
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    window._caduceus?.toast('Translation preferences reset.')
  }

  /** Primary mailbox plus exact aliases available as safe From identities. */
  get senderAddresses() {
    return [...new Set([this.$primaryEmail, ...this.$aliases].map(address => String(address || '').toLowerCase()).filter(Boolean))]
  }

  /**
   * Restore MFA setup wizard state during rendering.
   *
   * Used as a template-side expression (always returns `false`) to trigger
   * state restoration from the window-level cache without a dedicated mount hook.
   *
   * @returns {false}
   */
  restoreMfaSetupStateForRender() { this.restoreMfaSetupState(); return false }

  /**
   * Synchronise the notification state fields with the browser's current
   * capabilities and permission status.
   *
   * @returns {void}
   */
  syncNotificationState() {
    this.$notificationSupported = this.canUseNotifications
    this.$notificationPermission = this.$notificationSupported ? Notification.permission : 'unsupported'
  }

  /**
   * Persist the MFA device setup wizard state to a window-level cache so it
   * survives component re-renders.
   *
   * @returns {void}
   */
  saveMfaSetupState() { window[this._mfaSetupStateKey] = { showAddDevice: this.$showAddDevice, deviceProvision: this.$deviceProvision, newDeviceName: this.$newDeviceName, newDeviceCode: this.$newDeviceCode, deviceError: this.$deviceError } }

  /**
   * Restore the MFA device setup wizard state from the window-level cache.
   *
   * Only overwrites fields that are currently empty/falsy to avoid clobbering
   * user input.
   *
   * @returns {void}
   */
  restoreMfaSetupState() {
    const state = window[this._mfaSetupStateKey]; if (!state) return
    this.$showAddDevice = state.showAddDevice
    if (state.deviceProvision) this.$deviceProvision = state.deviceProvision
    if (!this.$newDeviceName && state.newDeviceName) this.$newDeviceName = state.newDeviceName
    if (!this.$newDeviceCode && state.newDeviceCode) this.$newDeviceCode = state.newDeviceCode
    if (!this.$deviceError && state.deviceError) this.$deviceError = state.deviceError
  }

  /**
   * Clear the cached MFA setup wizard state.
   *
   * @returns {void}
   */
  clearMfaSetupState() { delete window[this._mfaSetupStateKey] }

  /**
   * Read a field value for controls which the settings template can add
   * without coupling the API behavior to a specific input component.
   *
   * @param {string} selector
   * @param {string} [fallback='']
   * @returns {string}
   */
  formValue(selector, fallback = '') {
    if (typeof document === 'undefined') return String(fallback ?? '').trim()
    const host = document.querySelector(selector)
    const control = host?.matches('input, textarea, select')
      ? host
      : host?.querySelector('input, textarea, select')
    const value = /** @type {HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null | undefined} */ (control)?.value
      ?? /** @type {{ value?: unknown } | null} */ (host)?.value
    return String(value ?? fallback ?? '').trim()
  }

  /**
   * @param {ArrayBuffer} value
   * @returns {string}
   */
  /** @param {string} value @returns {ArrayBuffer} */
  base64UrlToBuffer(value) {
    const padded = value.replace(/-/g, '+').replace(/_/g, '/')
    const raw = atob(padded + '='.repeat((4 - (padded.length % 4)) % 4))
    const bytes = new Uint8Array(raw.length)
    for (let index = 0; index < raw.length; index += 1) bytes[index] = raw.charCodeAt(index)
    return bytes.buffer
  }

  bufferToBase64Url(value) {
    const bytes = new Uint8Array(value)
    let raw = ''
    for (const byte of bytes) raw += String.fromCharCode(byte)
    return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  }


  /**
   * Prove a factor again before changing a credential.
   *
   * Changing a credential is the action an account takeover needs, so the
   * server will not accept one on the strength of a session established at
   * some point in the past. This runs a fresh authentication transaction and
   * returns the identifier that proves it. The transaction is single use.
   *
   * @returns {Promise<string>} the satisfied transaction, or '' if it was not
   */
  async stepUp() {
    const apiFetch = window._caduceus?.apiFetch
    const address = window._caduceus?.auth?.email ?? sessionStorage.getItem('caduceus_email')
    if (!apiFetch || !address) return ''
    const code = this.formValue('[data-settings-stepup-code]', this.$stepUpCode).trim()
    const password = this.formValue('[data-settings-stepup-password]', this.$stepUpPassword)
    if (!password || !code) {
      this.$stepUpError = 'Confirm it is you: enter your password and one recovery code.'
      this.$showStepUp = true
      return ''
    }
    const begun = await (await apiFetch('/auth/login/begin', {
      method: 'POST', body: JSON.stringify({ address }),
    }))?.json()
    if (!begun?.transactionId) { this.$stepUpError = 'Unable to re-authenticate.'; return '' }
    await apiFetch('/auth/login/password', {
      method: 'POST',
      body: JSON.stringify({ transactionId: begun.transactionId, password }),
    })
    const verified = await (await apiFetch('/auth/login/recovery', {
      method: 'POST',
      body: JSON.stringify({ transactionId: begun.transactionId, code }),
    }))?.json()
    if (!verified?.satisfied) {
      this.$stepUpError = 'That password and code did not confirm your identity.'
      this.$showStepUp = true
      return ''
    }
    this.$stepUpCode = this.$stepUpPassword = this.$stepUpError = ''
    this.$showStepUp = false
    return begun.transactionId
  }

  /** Reload the caller's own registered passkeys. */
  async refreshPasskeys() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const res = await apiFetch('/auth/credentials/passkey')
    const data = res?.ok ? await res.json() : null
    this.$mfaDevices = Array.isArray(data?.passkeys) ? data.passkeys : []
  }

  /**
   * @param {Credential|null} credential
   * @returns {Record<string, unknown>}
   */
  serializeCredential(credential) {
    const publicKeyCredential = /** @type {PublicKeyCredential|null} */ (credential)
    if (!publicKeyCredential) throw new Error('No passkey credential returned.')
    const response = /** @type {AuthenticatorResponse & Record<string, unknown>} */ (publicKeyCredential.response)
    /** @type {Record<string, unknown>} */
    const serializedResponse = {}
    for (const key of ['clientDataJSON', 'attestationObject', 'authenticatorData', 'signature', 'userHandle']) {
      if (response[key]) serializedResponse[key] = this.bufferToBase64Url(/** @type {ArrayBuffer} */ (response[key]))
    }
    if (typeof /** @type {AuthenticatorAttestationResponse} */ (/** @type {unknown} */ (response)).getTransports === 'function') {
      serializedResponse.transports = /** @type {AuthenticatorAttestationResponse} */ (/** @type {unknown} */ (response)).getTransports()
    }
    return {
      id: publicKeyCredential.id,
      type: publicKeyCredential.type,
      rawId: this.bufferToBase64Url(publicKeyCredential.rawId),
      response: serializedResponse,
    }
  }

  /**
   * Re-fetch push notification subscriptions from the API and sync state.
   *
   * @async
   * @returns {Promise<void>}
   */
  async refreshPushSubscriptions() {
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const res = await apiFetch('/notifications/subscriptions'); this.$pushSubscriptions = res?.ok ? await res.json() : []; this.syncNotificationState()
  }

  /**
   * Enable Web Push notifications for this device.
   *
   * Requests notification permission, retrieves the VAPID public key from the
   * server, subscribes the service worker to push, and saves the subscription.
   *
   * @async
   * @returns {Promise<void>}
   */
  async enableNotifications() {
    this.$notificationError = ''; this.syncNotificationState()
    if (!this.$notificationSupported) { this.$notificationError = 'This browser cannot receive Web Push notifications.'; return }
    this.$notificationLoading = true
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$notificationLoading = false; return }
    try {
      const permission = await Notification.requestPermission(); this.$notificationPermission = permission
      if (permission !== 'granted') { this.$notificationError = 'Notifications were not allowed.'; return }
      const keyRes = await apiFetch('/notifications/vapid-public-key'); if (!keyRes?.ok) throw new Error('Unable to load notification key')
      const { publicKey } = await keyRes.json()
      const cb = (s) => { const p = '='.repeat((4 - s.length % 4) % 4); const b = (s + p).replace(/-/g, '+').replace(/_/g, '/'); return Uint8Array.from([...atob(b)].map(c => c.charCodeAt(0))) }
      const reg = await navigator.serviceWorker.ready
      const sub = (await reg.pushManager.getSubscription()) || await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: cb(publicKey) })
      const saveRes = await apiFetch('/notifications/subscriptions', { method: 'POST', body: JSON.stringify(sub.toJSON()) })
      if (!saveRes?.ok) throw new Error('Unable to save notification subscription')
      await this.refreshPushSubscriptions(); window._caduceus?.toast('Notifications enabled.')
    } catch { this.$notificationError = 'Unable to enable notifications on this device.' }
    finally { this.$notificationLoading = false }
  }

  /**
   * Disable Web Push notifications for this device.
   *
   * Deletes the subscription from the server and unsubscribes the local push
   * subscription.
   *
   * @async
   * @returns {Promise<void>}
   */
  async disableNotifications() {
    this.$notificationError = ''; this.syncNotificationState(); if (!this.$notificationSupported) return
    this.$notificationLoading = true; const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$notificationLoading = false; return }
    try {
      const reg = await navigator.serviceWorker.ready; const sub = await reg.pushManager.getSubscription()
      if (sub) { await apiFetch('/notifications/subscriptions', { method: 'DELETE', body: JSON.stringify({ endpoint: sub.endpoint }) }); await sub.unsubscribe() }
      await this.refreshPushSubscriptions(); window._caduceus?.toast('Notifications disabled.')
    } catch { this.$notificationError = 'Unable to disable notifications on this device.' }
    finally { this.$notificationLoading = false }
  }

  /**
   * Add a new user (admin only).
   *
   * Validates required fields, posts to `/users`, and refreshes the user list.
   *
   * @async
   * @returns {Promise<void>}
   */
  async addUser() {
    if (!this.$newEmail || !this.$newDomains) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const res = await apiFetch('/users', { method: 'POST', body: JSON.stringify({ email: this.$newEmail, domains: this.$newDomains.split(',').map(d => d.trim()).filter(Boolean), role: this.$newRole }) })
    if (res?.ok) { this.$showAddUser = false; this.$newEmail = this.$newDomains = ''; this.$newRole = 'viewer'; const ur = await apiFetch('/users'); this.$users = ur?.ok ? await ur.json() : []; window._caduceus?.toast('User added. Issue an enrollment code when they are ready.') }
    else { const data = await res?.json(); window._caduceus?.toast(data?.error || 'Failed to add user.') }
  }

  /**
   * Issuing another mailbox's first credentials is an operator act, not an
   * application one: it needs the deployment's key boundary, which only the
   * command line has. The UI says so rather than offering a button that
   * cannot work.
   *
   * @param {string} email
   * @returns {Promise<void>}
   */
  async issueAdminRecovery(email) {
    this.$adminRecoveryCodes = {
      ...this.$adminRecoveryCodes,
      [email]: {
        operatorCommand: `caduceus auth credentials --address ${email} --password '<generated>'`,
        note: 'Run this on the server. It prints a recovery code set once; hand it to the mailbox owner out of band.',
      },
    }
    window._caduceus?.toast('Run the operator command shown to issue credentials.')
  }

  /**
   * Append a condition to the new-rule form.
   *
   * Conditions define when a rule should fire (e.g. `from contains "example.com"`).
   *
   * @returns {Promise<void>}
   */
  async addCondition() {
    this._syncRuleBuilderFields()
    const value = this.$condValue.trim()
    if (!value) return
    if (this.$condField === 'hasAttachment' && !['true', 'false'].includes(value.toLowerCase())) {
      window._caduceus?.toast('Has attachment must be true or false.')
      return
    }
    this.$newConditions = [...this.$newConditions, { _id: ++this._condSeq, field: this.$condField, op: this.$condOp, value }]
    this.$condValue = ''
    await this._rerenderRules()
  }

  /**
   * Remove a condition from the new-rule form by its internal ID.
   *
   * @param {Event} _event
   * @param {number} id - The internal `_id` of the condition to remove
   * @returns {Promise<void>}
   */
  async removeCondition(_event, id) { this.$newConditions = this.$newConditions.filter(c => c._id !== id); await this._rerenderRules() }

  /**
   * Append an action to the new-rule form.
   *
   * Actions define what happens when a rule's conditions are met (move to
   * folder, forward to address, or delete).
   *
   * @returns {Promise<void>}
   */
  async addAction() {
    this._syncRuleBuilderFields()
    if (this.$actionType === 'folder') {
      const folder = this.normalizeFolderName(this.$actionFolder)
      if (!folder) { window._caduceus?.toast('Enter a folder name.'); return }
      this.$newActions = [...this.$newActions, { _id: ++this._actSeq, type: 'folder', folder }]
      this.$actionFolder = ''
      await this._rerenderRules()
      return
    }
    if (this.$actionType === 'forward') {
      const to = this.$actionTo.trim().toLowerCase()
      if (!to) { window._caduceus?.toast('Enter a forwarding address.'); return }
      this.$newActions = [...this.$newActions, { _id: ++this._actSeq, type: 'forward', to }]
      this.$actionTo = ''
      await this._rerenderRules()
      return
    }
    const stateAction = /^(markRead|star)(True|False)$/.exec(this.$actionType)
    this.$newActions = [...this.$newActions, stateAction
      ? { _id: ++this._actSeq, type: stateAction[1], value: stateAction[2] === 'True' }
      : { _id: ++this._actSeq, type: 'delete' }]
    await this._rerenderRules()
  }

  /**
   * Remove an action from the new-rule form by its internal ID.
   *
   * @param {Event} _event
   * @param {number} id - The internal `_id` of the action to remove
   * @returns {Promise<void>}
   */
  async removeAction(_event, id) { this.$newActions = this.$newActions.filter(a => a._id !== id); await this._rerenderRules() }

  /** @returns {Promise<void>} */
  async toggleAddRule() {
    this.$showAddRule = !this.$showAddRule
    if (this.$showAddRule && !this.$newRuleMailbox) this.$newRuleMailbox = this.senderAddresses[0] || ''
    if (!this.$showAddRule) this.resetRuleForm()
    await this._rerenderRules()
  }

  /**
   * Normalise user-created folder names for rule actions.
   *
   * Custom folders are intentionally plug-and-play: a rule can move mail to
   * any non-empty folder name, and the folder appears once mail lands there.
   *
   * @param {string} value
   * @returns {string}
   */
  normalizeFolderName(value) {
    return String(value || '')
      .trim()
      .toLowerCase()
      .replace(/\s+/g, '-')
      .replace(/[^a-z0-9/_-]/g, '')
      .replace(/\/+/g, '/')
      .replace(/^\/+|\/+$/g, '')
      .slice(0, 60)
  }

  /**
   * Reset all fields in the new-rule form to their defaults.
   *
   * @returns {void}
   */
  resetRuleForm() {
    ;(/** @type {any} */ (window)).__caduceusRuleEditId = ''
    this.$ruleEditId = this.$newRuleName = this.$newRuleMailbox = ''
    this.$newConditionMatch = 'all'; this.$newStopProcessing = false
    this.$newConditions = []; this.$newActions = []; this.$condField = 'from'; this.$condOp = 'contains'; this.$condValue = ''
    this.$actionType = 'folder'; this.$actionFolder = this.$actionTo = ''
    this.$previewFrom = this.$previewSubject = this.$previewBody = this.$rulePreviewResult = ''
    this.$previewHasAttachment = false; this.$showAddRule = false
  }

  /**
   * Persist the new rule to the API.
   *
   * Strips internal `_id` fields from conditions and actions before sending.
   * On success, refreshes the rules list and resets the form.
   *
   * @async
   * @returns {Promise<void>}
   */
  async saveRule() {
    this._syncRuleBuilderFields()
    this.$newRuleName = this.$newRuleName.trim()
    this.$newRuleMailbox = this.$newRuleMailbox.trim().toLowerCase()
    if (!this.$newRuleName || !this.$newRuleMailbox || this.$newActions.length === 0) { window._caduceus?.toast('Name, mailbox, and at least one action are required.'); return }
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const editId = String((/** @type {any} */ (window)).__caduceusRuleEditId || this.$ruleEditId || '')
    const payload = { name: this.$newRuleName, mailbox: this.$newRuleMailbox, enabled: true, stopProcessing: this.$newStopProcessing, conditionMatch: this.$newConditionMatch, conditions: this.$newConditions.map(({ _id, ...c }) => c), actions: this.$newActions.map(({ _id, ...a }) => a) }
    const res = await apiFetch(editId ? `/rules/${editId}` : '/rules', { method: editId ? 'PUT' : 'POST', body: JSON.stringify(payload) })
    if (res?.ok) { await this.reloadRules(); window._caduceus?.toast(editId ? 'Rule updated.' : 'Rule saved.'); this.resetRuleForm(); await this._rerenderRules() }
    else { const data = await res?.json(); window._caduceus?.toast(data?.error || 'Failed to save rule.') }
  }

  /**
   * Delete a rule by ID.
   *
   * Prompts for confirmation before sending the DELETE request.
   *
   * @async
   * @param {string} id - The rule ID to delete
   * @returns {Promise<void>}
   */
  async deleteRule(_event, id) { if (!confirm('Delete this rule?')) return; const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return; const res = await apiFetch(`/rules/${id}`, { method: 'DELETE' }); if (res?.ok) { this.$rules = this.$rules.filter(r => r.id !== id); window._caduceus?.toast('Rule deleted.'); await this._rerenderRules() } }

  /**
   * Toggle a rule's enabled / disabled state.
   *
   * @async
   * @param {string} id - The rule ID
   * @param {boolean} enabled - The current enabled state (will be toggled)
   * @returns {Promise<void>}
   */
  async toggleRule(_event, id, enabled) { const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return; await apiFetch(`/rules/${id}`, { method: 'PUT', body: JSON.stringify({ enabled: !enabled }) }); await this.reloadRules(); await this._rerenderRules() }

  async reloadRules() { const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return; const response = await apiFetch('/rules'); this.$rules = response?.ok ? await response.json() : [] }

  canMoveRule(id, delta) { const index = this.$rules.findIndex(rule => rule.id === id); return index >= 0 && index + Number(delta) >= 0 && index + Number(delta) < this.$rules.length }

  async moveRule(_event, id, delta) {
    const index = this.$rules.findIndex(rule => rule.id === id); const target = index + Number(delta)
    if (index < 0 || target < 0 || target >= this.$rules.length) return
    const next = [...this.$rules]; const [moved] = next.splice(index, 1); next.splice(target, 0, moved)
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const response = await apiFetch('/rules/order', { method: 'PUT', body: JSON.stringify({ ids: next.map(rule => rule.id) }) })
    if (!response?.ok) { window._caduceus?.toast('Could not reorder rules.'); return }
    this.$rules = next; await this._rerenderRules()
  }

  async editRule(_event, rule) {
    ;(/** @type {any} */ (window)).__caduceusRuleEditId = String(rule?.id || '')
    this.$ruleEditId = String(rule?.id || ''); this.$newRuleName = String(rule?.name || '')
    this.$newRuleMailbox = String(rule?.mailbox || this.senderAddresses[0] || '')
    this.$newConditionMatch = rule?.conditionMatch === 'any' ? 'any' : 'all'; this.$newStopProcessing = Boolean(rule?.stopProcessing)
    this.$newConditions = (rule?.conditions || []).map(condition => ({ ...condition, _id: ++this._condSeq }))
    this.$newActions = (rule?.actions || []).map(action => ({ ...action, _id: ++this._actSeq }))
    this.$showAddRule = true; this.$rulePreviewResult = ''; await this._rerenderRules()
  }

  async previewRule() {
    this._syncRuleBuilderFields()
    if (!this.$newRuleName.trim() || !this.$newRuleMailbox || this.$newActions.length === 0) { this.$rulePreviewResult = 'Add a name, mailbox, and action before previewing.'; await this._rerenderRules(); return }
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const rule = { name: this.$newRuleName.trim(), mailbox: this.$newRuleMailbox, enabled: true, stopProcessing: this.$newStopProcessing, conditionMatch: this.$newConditionMatch, conditions: this.$newConditions.map(({ _id, ...condition }) => condition), actions: this.$newActions.map(({ _id, ...action }) => action) }
    const response = await apiFetch('/rules/preview', { method: 'POST', body: JSON.stringify({ rule, message: { from: this.$previewFrom, to: this.$newRuleMailbox, subject: this.$previewSubject, body: this.$previewBody, hasAttachment: this.$previewHasAttachment } }) })
    const result = await response?.json()
    this.$rulePreviewResult = response?.ok
      ? result.matched ? `Match · ${result.plannedActions.map(action => this.describeAction(action)).join(', ')}` : 'No match · no actions would run'
      : result?.error || 'Preview failed.'
    await this._rerenderRules()
  }

  _ruleControlValue(selector, fallback = '') {
    const host = document.querySelector(selector); const control = host?.matches('input, textarea, select') ? host : host?.querySelector('input, textarea, select')
    return String(/** @type {HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null} */ (control)?.value ?? fallback)
  }

  _syncRuleBuilderFields() {
    this.$newRuleName = this._ruleControlValue('[data-rule-name]', this.$newRuleName)
    this.$newRuleMailbox = this._ruleControlValue('[data-rule-mailbox]', this.$newRuleMailbox).toLowerCase()
    this.$newConditionMatch = this._ruleControlValue('[data-rule-match]', this.$newConditionMatch)
    this.$condField = this._ruleControlValue('[data-rule-condition-field]', this.$condField)
    this.$condOp = this._ruleControlValue('[data-rule-condition-op]', this.$condOp)
    this.$condValue = this._ruleControlValue('[data-rule-condition-value]', this.$condValue)
    this.$actionType = this._ruleControlValue('[data-rule-action-type]', this.$actionType)
    this.$actionFolder = this._ruleControlValue('[data-rule-action-folder]', this.$actionFolder)
    this.$actionTo = this._ruleControlValue('[data-rule-action-to]', this.$actionTo)
    this.$previewFrom = this._ruleControlValue('[data-rule-preview-from]', this.$previewFrom)
    this.$previewSubject = this._ruleControlValue('[data-rule-preview-subject]', this.$previewSubject)
    this.$previewBody = this._ruleControlValue('[data-rule-preview-body]', this.$previewBody)
    this.$newStopProcessing = Boolean(/** @type {HTMLInputElement|null} */ (document.querySelector('[data-rule-stop]'))?.checked)
    this.$previewHasAttachment = Boolean(/** @type {HTMLInputElement|null} */ (document.querySelector('[data-rule-preview-attachment]'))?.checked)
  }

  /** Preserve native DuVay values across Tachyon's document rerender. */
  captureRuleValue(event, key) {
    const value = String(/** @type {{ target?: { value?: unknown } }} */ (event)?.target?.value ?? '')
    if (key === 'name') this.$newRuleName = value
    else if (key === 'mailbox') this.$newRuleMailbox = value
    else if (key === 'match') this.$newConditionMatch = value
    else if (key === 'conditionField') this.$condField = value
    else if (key === 'conditionOp') this.$condOp = value
    else if (key === 'conditionValue') this.$condValue = value
    else if (key === 'actionType') this.$actionType = value
    else if (key === 'actionFolder') this.$actionFolder = value
    else if (key === 'actionTo') this.$actionTo = value
    else if (key === 'previewFrom') this.$previewFrom = value
    else if (key === 'previewSubject') this.$previewSubject = value
    else if (key === 'previewBody') this.$previewBody = value
    setTimeout(() => this._restoreRuleBuilderFields(), 0)
  }

  captureRuleToggle(event, key) {
    const checked = Boolean(/** @type {{ target?: { checked?: unknown } }} */ (event)?.target?.checked)
    if (key === 'stop') this.$newStopProcessing = checked
    if (key === 'attachment') this.$previewHasAttachment = checked
    setTimeout(() => this._restoreRuleBuilderFields(), 0)
  }

  _restoreRuleBuilderFields() {
    const setValue = (selector, value) => {
      const host = /** @type {HTMLElement & { value?: string } | null} */ (document.querySelector(selector))
      const control = /** @type {HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null | undefined} */ (host?.matches('input, textarea, select') ? host : host?.querySelector('input, textarea, select'))
      if (host && 'value' in host) host.value = value
      if (control) control.value = value
    }
    for (const [selector, value] of [
      ['[data-rule-name]', this.$newRuleName], ['[data-rule-mailbox]', this.$newRuleMailbox],
      ['[data-rule-match]', this.$newConditionMatch], ['[data-rule-condition-field]', this.$condField],
      ['[data-rule-condition-op]', this.$condOp], ['[data-rule-condition-value]', this.$condValue],
      ['[data-rule-action-type]', this.$actionType], ['[data-rule-action-folder]', this.$actionFolder],
      ['[data-rule-action-to]', this.$actionTo], ['[data-rule-preview-from]', this.$previewFrom],
      ['[data-rule-preview-subject]', this.$previewSubject], ['[data-rule-preview-body]', this.$previewBody],
    ]) setValue(selector, value)
    const stop = /** @type {HTMLInputElement|null} */ (document.querySelector('[data-rule-stop]'))
    const attachment = /** @type {HTMLInputElement|null} */ (document.querySelector('[data-rule-preview-attachment]'))
    if (stop) stop.checked = this.$newStopProcessing
    if (attachment) attachment.checked = this.$previewHasAttachment
  }

  async _rerenderRules() {
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))
    this._restoreRuleBuilderFields()
  }

  /**
   * Build a human-readable description of a rule's conditions.
   *
   * @param {{ conditions?: Array<{ field: string, op: string, value: string }>, conditionMatch?: string }} rule - Rule object
   * @returns {string} e.g. `from contains "example.com" all subject contains "invoice"`
   */
  describeConditions(rule) { if (!rule.conditions?.length) return 'Always'; return rule.conditions.map(c => `${c.field} ${c.op} "${c.value}"`).join(` ${rule.conditionMatch} `) }

  /**
   * Build a human-readable description of a rule's actions.
   *
   * @param {{ type: string, folder?: string, to?: string, value?: boolean }} a - Rule action
   * @returns {string} e.g. `-> folder "archive", -> forward to user@example.com`
   */
  describeAction(a) { return a.type === 'folder' ? `→ folder "${a.folder}"` : a.type === 'forward' ? `→ forward to ${a.to}` : a.type === 'markRead' ? `→ mark ${a.value ? 'read' : 'unread'}` : a.type === 'star' ? `→ ${a.value ? 'star' : 'unstar'}` : '→ delete' }

  describeActions(rule) { return (rule.actions || []).map(a => this.describeAction(a)).join(', ') }

  /**
   * Start passkey enrollment by requesting browser registration options.
   *
   * Toggles the wizard closed if it is already open.
   *
   * @async
   * @returns {Promise<void>}
   */
  async startAddDevice() {
    if (this.$showAddDevice) { this.$showAddDevice = false; this.$deviceProvision = null; this.$newDeviceName = this.$newDeviceCode = this.$deviceError = ''; this.clearMfaSetupState(); return }
    this.$showAddDevice = true; this.$deviceProvision = null; this.saveMfaSetupState()
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const stepUpTransactionId = await this.stepUp()
    if (!stepUpTransactionId) { this.$showAddDevice = false; this.clearMfaSetupState(); return }
    const res = await apiFetch('/auth/credentials/passkey/begin', {
      method: 'POST', body: JSON.stringify({ stepUpTransactionId }),
    })
    if (res?.ok) { this.$deviceProvision = await res.json(); this.saveMfaSetupState() }
    else { window._caduceus?.toast('Failed to start device setup.'); this.$showAddDevice = false; this.clearMfaSetupState() }
  }

  /**
   * Complete passkey enrollment using the browser's WebAuthn ceremony.
   *
   * On success, closes the wizard, clears cached state, and refreshes the
   * devices list.
   *
   * @async
   * @returns {Promise<void>}
   */
  async confirmDevice() {
    const provision = /** @type {{ challenge?: string, relyingPartyId?: string, principalId?: string }|null} */ (this.$deviceProvision)
    if (!provision?.challenge || !provision.relyingPartyId) { this.$deviceError = 'Passkey setup has expired. Start again.'; this.saveMfaSetupState(); return }
    if (typeof PublicKeyCredential === 'undefined') { this.$deviceError = 'Passkeys are not supported on this device.'; return }
    this.$deviceLoading = true; this.$deviceError = ''
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    try {
      const address = window._caduceus?.auth?.email ?? sessionStorage.getItem('caduceus_email') ?? 'mailbox'
      const credential = /** @type {PublicKeyCredential|null} */ (
        await navigator.credentials.create({
          publicKey: {
            challenge: this.base64UrlToBuffer(provision.challenge),
            rp: { id: provision.relyingPartyId, name: 'CADUCEUS' },
            user: {
              id: this.base64UrlToBuffer(
                (provision.principalId ?? address).replace(/[^A-Za-z0-9_-]/g, ''),
              ),
              name: address,
              displayName: this.$newDeviceName.trim() || address,
            },
            pubKeyCredParams: [{ type: 'public-key', alg: -7 }, { type: 'public-key', alg: -257 }],
            authenticatorSelection: { userVerification: 'required', residentKey: 'preferred' },
          },
        })
      )
      if (!credential) { this.$deviceError = 'No passkey credential returned.'; return }
      const attestation = /** @type {AuthenticatorAttestationResponse} */ (credential.response)
      const res = await apiFetch('/auth/credentials/passkey/finish', {
        method: 'POST',
        body: JSON.stringify({
          attestationObject: this.bufferToBase64Url(attestation.attestationObject),
          clientDataJSON: this.bufferToBase64Url(attestation.clientDataJSON),
        }),
      })
      if (res?.ok) { window._caduceus?.toast('Passkey added.'); this.$showAddDevice = false; this.$deviceProvision = null; this.$newDeviceName = this.$newDeviceCode = ''; this.clearMfaSetupState(); await this.refreshPasskeys() }
      else { const data = await res?.json(); this.$deviceError = data?.error || 'Unable to save passkey.'; this.saveMfaSetupState() }
    } catch (e) {
      this.$deviceError = /** @type {Error} */ (e).name === 'NotAllowedError' ? 'Passkey creation was cancelled.' : 'Unable to add passkey.'
      this.saveMfaSetupState()
    } finally { this.$deviceLoading = false }
  }

  /** @returns {Promise<void>} */
  async startTotpEnrollment() {
    if (this.$totpEnrollment) { this.$totpEnrollment = null; this.$totpCode = ''; this.$totpError = ''; return }
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    this.$totpLoading = true; this.$totpError = ''
    try {
      const stepUpTransactionId = await this.stepUp()
      if (!stepUpTransactionId) { this.$totpError = 'Confirm it is you before enrolling an authenticator.'; return }
      const res = await apiFetch('/auth/credentials/totp/enroll', {
        method: 'POST', body: JSON.stringify({ stepUpTransactionId }),
      })
      const data = await res?.json()
      if (!res?.ok) { this.$totpError = data?.error || 'Unable to start authenticator enrollment.'; return }
      this.$totpEnrollment = data
    } catch { this.$totpError = 'Network error.' }
    finally { this.$totpLoading = false }
  }

  /** @returns {Promise<void>} */
  async confirmTotpEnrollment() {
    const enrollment = /** @type {{ secret?: string }|null} */ (this.$totpEnrollment)
    this.$totpCode = this.formValue('[data-settings-totp-code]', this.$totpCode)
    this.$totpName = this.formValue('[data-settings-totp-name]', this.$totpName)
    if (!enrollment?.secret || !/^\d{6}$/.test(this.$totpCode)) { this.$totpError = 'Enter the current 6-digit authenticator code.'; return }
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    this.$totpLoading = true; this.$totpError = ''
    try {
      const res = await apiFetch('/auth/credentials/totp/activate', { method: 'POST', body: JSON.stringify({ code: this.$totpCode }) })
      const data = await res?.json()
      if (!res?.ok) { this.$totpError = data?.error || 'Unable to verify authenticator code.'; return }
      this.$totpEnrollment = null; this.$totpCode = ''
      await this.refreshPasskeys()
      window._caduceus?.toast('Authenticator app enabled.')
    } catch { this.$totpError = 'Network error.' }
    finally { this.$totpLoading = false }
  }

  /**
   * Remove an MFA device by ID.
   *
   * Prompts for confirmation before deleting.
   *
   * @async
   * @param {string} id - The MFA device ID to remove
   * @returns {Promise<void>}
   */
  async deleteMfaDevice(id) {
    if (!confirm('Remove this passkey?')) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const stepUpTransactionId = await this.stepUp()
    if (!stepUpTransactionId) return
    const res = await apiFetch('/auth/credentials/passkey', {
      method: 'DELETE', body: JSON.stringify({ stepUpTransactionId, credentialId: id }),
    })
    if (res?.ok) { await this.refreshPasskeys(); window._caduceus?.toast('Passkey removed.') }
    else { window._caduceus?.toast('Unable to remove that passkey.') }
  }

  /**
   * Establish or change the account password after a recent verified passkey.
   *
   * @returns {Promise<void>}
   */
  async updatePassword() {
    this.$newPassword = this.formValue('[data-settings-new-password]', this.$newPassword)
    this.$confirmPassword = this.formValue('[data-settings-confirm-password]', this.$confirmPassword)
    if (!this.$newPassword) { this.$passwordError = 'Password is required.'; return }
    if (this.$confirmPassword && this.$confirmPassword !== this.$newPassword) { this.$passwordError = 'Passwords do not match.'; return }
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    this.$passwordSaving = true; this.$passwordError = ''
    try {
      const stepUpTransactionId = await this.stepUp()
      if (!stepUpTransactionId) { this.$passwordError = 'Confirm it is you before changing your password.'; return }
      const res = await apiFetch('/auth/credentials/password', {
        method: 'PUT',
        body: JSON.stringify({ stepUpTransactionId, password: this.$newPassword }),
      })
      const data = await res?.json()
      if (!res?.ok) { this.$passwordError = data?.error || 'Unable to update password.'; return }
      this.$newPassword = this.$confirmPassword = ''
      window._caduceus?.toast('Password updated.')
    } catch { this.$passwordError = 'Network error.' }
    finally { this.$passwordSaving = false }
  }

  /**
   * Generate a replacement set of one-time recovery codes. Plaintext codes
   * are kept only in component memory so the view can present them once.
   *
   * @returns {Promise<void>}
   */
  async generateRecoveryCodes() {
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    this.$recoveryLoading = true; this.$recoveryError = ''; this.$recoveryCodes = []
    try {
      const stepUpTransactionId = await this.stepUp()
      if (!stepUpTransactionId) { this.$recoveryError = 'Confirm it is you before replacing your recovery codes.'; return }
      const res = await apiFetch('/auth/credentials/recovery', {
        method: 'POST', body: JSON.stringify({ stepUpTransactionId }),
      })
      const data = await res?.json()
      if (!res?.ok) { this.$recoveryError = data?.error || 'Unable to generate recovery codes.'; return }
      this.$recoveryCodes = Array.isArray(data?.codes) ? data.codes : []
      window._caduceus?.toast('Recovery codes replaced.')
    } catch { this.$recoveryError = 'Network error.' }
    finally { this.$recoveryLoading = false }
  }

  /** @returns {void} */
  dismissRecoveryCodes() { this.$recoveryCodes = []; this.$recoveryError = '' }

  /** Delegate rich clipboard paste into the raw HTML source editor. */
  bindSignatureEditor() {
    if (window._caduceusSignaturePasteHandler) {
      document.removeEventListener('paste', window._caduceusSignaturePasteHandler, true)
    }
    if (window._caduceusSignatureInputHandler) {
      document.removeEventListener('input', window._caduceusSignatureInputHandler, true)
    }
    window._caduceusSignaturePasteHandler = event => {
      const textarea = /** @type {HTMLTextAreaElement | null} */ (
        event.target instanceof Element ? event.target.closest('[data-signature-html] textarea, textarea[data-signature-html]') : null
      )
      if (!textarea || !event.clipboardData) return
      const rich = signatureHTMLFromClipboard(
        event.clipboardData.getData('text/plain'),
        event.clipboardData.getData('text/html'),
      )
      if (!rich) return
      event.preventDefault()
      const start = textarea.selectionStart ?? textarea.value.length
      const end = textarea.selectionEnd ?? start
      textarea.setRangeText(rich, start, end, 'end')
      textarea.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertFromPaste', data: rich }))
    }
    window._caduceusSignatureInputHandler = event => {
      const textarea = /** @type {HTMLTextAreaElement | null} */ (
        event.target instanceof Element ? event.target.closest('[data-signature-html] textarea, textarea[data-signature-html]') : null
      )
      if (!textarea) return
      this.$signatureText = textarea.value
      this.renderSignaturePreview(textarea.value)
    }
    document.addEventListener('paste', window._caduceusSignaturePasteHandler, true)
    document.addEventListener('input', window._caduceusSignatureInputHandler, true)
  }

  /** Render only sanitized signature content into the settings preview. */
  renderSignaturePreview(value = this.$signatureText) {
    requestAnimationFrame(() => {
      const preview = document.querySelector('[data-signature-preview]')
      if (preview) preview.innerHTML = sanitizeSignatureHTML(value)
    })
  }

  /** Load one signature into the exact-address defaults editor. */
  editSignature(_event, signature) {
    ;(/** @type {any} */ (window)).__caduceusSignatureEditId = String(signature?.id || '')
    this.$signatureEditId = String(signature?.id || '')
    this.$signatureName = String(signature?.name || '')
    this.$signatureText = String(signature?.text || '')
    this.$signatureDefaults = Array.isArray(signature?.defaultFor) ? [...signature.defaultFor] : []
    this.$signatureError = ''
    this.renderSignaturePreview()
  }

  /** Reset signature management to create mode. */
  newSignature() {
    ;(/** @type {any} */ (window)).__caduceusSignatureEditId = ''
    this.$signatureEditId = this.$signatureName = this.$signatureText = this.$signatureError = ''
    this.$signatureDefaults = []
    this.renderSignaturePreview('')
  }

  /** Snapshot native DuVay fields once so reactive assignments cannot replace them mid-read. */
  _syncSignatureEditorFields() {
    const value = selector => {
      const host = document.querySelector(selector)
      const control = host?.matches('input, textarea') ? host : host?.querySelector('input, textarea')
      return String(/** @type {HTMLInputElement | HTMLTextAreaElement | null} */ (control)?.value ?? '')
    }
    const name = value('[data-signature-name]').trim()
    const text = value('[data-signature-html]')
    const defaults = Array.from(document.querySelectorAll('[data-signature-default]'))
      .filter(control => /** @type {HTMLInputElement} */ (control).checked)
      .map(control => String(/** @type {HTMLInputElement} */ (control).value))
    this.$signatureName = name
    this.$signatureText = text
    this.$signatureDefaults = defaults
  }

  /** Create or update one mailbox-owned rich signature. */
  async saveSignature() {
    this._syncSignatureEditorFields()
    if (!this.$signatureName || !this.$signatureText.trim()) {
      this.$signatureError = 'Name and signature content are required.'
      return
    }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$signatureSaving = true
    this.$signatureError = ''
    const editId = String((/** @type {any} */ (window)).__caduceusSignatureEditId || this.$signatureEditId || '')
    try {
      const res = await apiFetch(editId ? `/signatures/${editId}` : '/signatures', {
        method: editId ? 'PUT' : 'POST',
        body: JSON.stringify({ name: this.$signatureName, text: this.$signatureText, defaultFor: this.$signatureDefaults }),
      })
      const data = await res?.json()
      if (!res?.ok) { this.$signatureError = data?.error || 'Failed to save signature.'; return }
      this.$signatures = editId
        ? this.$signatures.map(signature => signature.id === editId ? data : signature)
        : [...this.$signatures, data]
      window._caduceus?.toast(editId ? 'Signature updated.' : 'Signature saved.')
      this.editSignature(null, data)
    } catch { this.$signatureError = 'Network error.' }
    finally { this.$signatureSaving = false }
  }

  /** Delete an exact-owner signature without exposing foreign IDs. */
  async deleteSignature(_event, id) {
    if (!confirm('Remove this signature?')) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const res = await apiFetch(`/signatures/${id}`, { method: 'DELETE' })
    if (res?.ok) {
      this.$signatures = this.$signatures.filter(signature => signature.id !== id)
      if (String((/** @type {any} */ (window)).__caduceusSignatureEditId || '') === id) this.newSignature()
      window._caduceus?.toast('Signature removed.')
    }
  }

  /** Load one template into the settings editor. */
  editTemplate(_event, template) {
    ;(/** @type {any} */ (window)).__caduceusTemplateEditId = String(template?.id || '')
    this.$templateEditId = String(template?.id || '')
    this.$templateName = String(template?.name || '')
    this.$templateSubject = String(template?.subject || '')
    this.$templateTo = String(template?.to || '')
    this.$templateCc = String(template?.cc || '')
    this.$templateText = String(template?.text || '')
    this.$templateError = ''
  }

  /** Reset the management editor to create mode. */
  newTemplate() {
    ;(/** @type {any} */ (window)).__caduceusTemplateEditId = ''
    this.$templateEditId = this.$templateName = this.$templateSubject = ''
    this.$templateTo = this.$templateCc = this.$templateText = this.$templateError = ''
  }

  /** Create or update a reusable rich composer template. */
  async saveManagedTemplate() {
    this._syncTemplateEditorFields()
    if (!this.$templateName.trim() || !this.$templateText.trim()) {
      this.$templateError = 'Name and template content are required.'
      return
    }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.$templateSaving = true
    this.$templateError = ''
    try {
      const editId = String((/** @type {any} */ (window)).__caduceusTemplateEditId || this.$templateEditId || '')
      const editing = Boolean(editId)
      const response = await apiFetch(editing ? `/templates/${editId}` : '/templates', {
        method: editing ? 'PUT' : 'POST',
        body: JSON.stringify({
          name: this.$templateName.trim(),
          subject: this.$templateSubject,
          to: this.$templateTo,
          cc: this.$templateCc,
          text: this.$templateText,
        }),
      })
      const data = await response?.json()
      if (!response?.ok) {
        this.$templateError = data?.error || 'Failed to save template.'
        return
      }
      this.$templates = editing
        ? this.$templates.map(item => item.id === data.id ? data : item).sort((a, b) => a.name.localeCompare(b.name))
        : [...this.$templates, data].sort((a, b) => a.name.localeCompare(b.name))
      this.editTemplate(null, data)
      window._caduceus?.toast(`Template “${data.name}” saved.`)
    } catch { this.$templateError = 'Network error.' }
    finally { this.$templateSaving = false }
  }

  /** Read native DuVay controls once, without rerendering between fields. */
  _syncTemplateEditorFields() {
    const value = selector => {
      const host = document.querySelector(selector)
      const control = host?.querySelector('input, textarea')
      return String(/** @type {HTMLInputElement | HTMLTextAreaElement | null} */ (control)?.value || '')
    }
    const snapshot = {
      name: value('[data-template-name]'),
      subject: value('[data-template-subject]'),
      to: value('[data-template-to]'),
      cc: value('[data-template-cc]'),
      text: value('[data-template-content]'),
    }
    this.$templateName = snapshot.name
    this.$templateSubject = snapshot.subject
    this.$templateTo = snapshot.to
    this.$templateCc = snapshot.cc
    this.$templateText = snapshot.text
  }

  /** Permanently remove one exact-owner template. */
  async deleteTemplate(_event, id) {
    if (!confirm('Delete this template permanently?')) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const response = await apiFetch(`/templates/${id}`, { method: 'DELETE' })
    if (!response?.ok) {
      const data = await response?.json()
      this.$templateError = data?.error || 'Failed to delete template.'
      return
    }
    this.$templates = this.$templates.filter(item => item.id !== id)
    if (this.$templateEditId === id || (/** @type {any} */ (window)).__caduceusTemplateEditId === id) this.newTemplate()
    window._caduceus?.toast('Template removed.')
  }
}
