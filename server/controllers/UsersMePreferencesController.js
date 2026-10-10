import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { Security } from '../shared/security/Security.js'
import { FontPreferences } from '../shared/users/FontPreferences.js'
import { NotificationSounds } from '../shared/users/NotificationSounds.js'
import { ThemePreferences } from '../shared/users/ThemePreferences.js'
import { LocalePreferences } from '../shared/users/LocalePreferences.js'
import { GrammarPreferences } from '../shared/users/GrammarPreferences.js'
import { CategoryMappingRules } from '../shared/mailbox/CategoryMappingRules.js'
import { UnreadBadgePreferences } from '../shared/users/UnreadBadgePreferences.js'
import { NewMailNotificationPreferences } from '../shared/users/NewMailNotificationPreferences.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'
import { KeyboardShortcuts } from '../shared/users/KeyboardShortcuts.js'
import { MailPreferences } from '../shared/users/MailPreferences.js'

@RequestRoute("/users/me/preferences")
@SignedIn
export class UsersMePreferencesController {
  static VALID_CAL_VIS = new Set(['private', 'busy', 'full'])

  static VALID_REMOTE_IMAGE_POLICIES = new Set(['block', 'allow'])

  static MAX_MAILBOX_NAME_LENGTH = 64

  static MAX_TRUSTED_IMAGE_SENDERS = 500

  /**
   * PUT /users/me/preferences
   *
   * Update the caller's preferences. Mailbox names are display-only labels for
   * domains already assigned to the caller.
   *
   * @param {object} params
   * @param {{ calendarVisibility?: string, mailboxNames?: Record<string, string>, categoryMappings?: Record<string, Record<string, string>>, remoteImagePolicy?: string, trustedImageSenders?: string[], interfaceFont?: string, composerFont?: string, interfaceLocale?: string, notificationSounds?: Record<string, string>, themePreferences?: Record<string, string>, githubProfileEnrichment?: boolean, grammarCheckEnabled?: boolean, grammarCheckLanguage?: string, grammarCheckDisabledRules?: string[], unreadBadgeScope?: string, unreadBadgeDomain?: string, newMailNotificationDomains?: string[], newMailNotificationExternalAccounts?: string[], newMailNotificationPrivacy?: string, newMailNotificationActions?: boolean, keyboardShortcutOverrides?: Record<string, string> }} params.body
   * @param {{ bearer?: { token: string } }} params.context
   * @returns {Promise<Record<string, unknown>>}
   */
  @PutRoute
  async put(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth, context }) => {
      const app = await getApp()
      const [docId, user] = await app.users.findByEmail(auth.email)
      if (!docId || !user) return Respond.r404('User not found')

      const next = { ...user }
      if (typeof body?.calendarVisibility === 'string') {
        if (!UsersMePreferencesController.VALID_CAL_VIS.has(body.calendarVisibility)) {
          return Respond.r400('calendarVisibility must be one of: private, busy, full')
        }
        /** @type {any} */ (next).calendarVisibility = body.calendarVisibility
      }
      if (body?.mailboxNames !== undefined) {
        const parsed = UsersMePreferencesController._normalizeMailboxNames(body.mailboxNames, user.domains)
        if (!parsed.ok) return parsed.forbidden ? Respond.r403(parsed.error) : Respond.r400(parsed.error)
        next.mailboxNames = parsed.mailboxNames
      }
      if (body?.categoryMappings !== undefined) {
        const parsed = CategoryMappingRules.validateCategoryMappings(body.categoryMappings, user.domains)
        if (!parsed.ok) return parsed.forbidden ? Respond.r403(parsed.error) : Respond.r400(parsed.error)
        next.categoryMappings = parsed.mappings
      }
      if (body?.remoteImagePolicy !== undefined) {
        if (typeof body.remoteImagePolicy !== 'string' || !UsersMePreferencesController.VALID_REMOTE_IMAGE_POLICIES.has(body.remoteImagePolicy)) {
          return Respond.r400('remoteImagePolicy must be one of: block, allow')
        }
        /** @type {any} */ (next).remoteImagePolicy = body.remoteImagePolicy
      }
      if (body?.trustedImageSenders !== undefined) {
        const parsed = UsersMePreferencesController._normalizeTrustedImageSenders(body.trustedImageSenders)
        if (!parsed.ok) return Respond.r400(parsed.error)
        next.trustedImageSenders = parsed.senders
      }
      if (body?.interfaceFont !== undefined) {
        if (!FontPreferences.isInterfaceFont(body.interfaceFont)) {
          return Respond.r400('interfaceFont must be one of: plex-sans, system, humanist, serif, monospace')
        }
        next.interfaceFont = body.interfaceFont
      }
      if (body?.composerFont !== undefined) {
        if (!FontPreferences.isComposerFont(body.composerFont)) {
          return Respond.r400('composerFont must be one of: interface, plex-sans, system, humanist, serif, monospace')
        }
        next.composerFont = body.composerFont
      }
      if (body?.interfaceLocale !== undefined) {
        const parsed = LocalePreferences.validateInterfaceLocale(body.interfaceLocale)
        if (!parsed.ok) return Respond.r400(parsed.error)
        next.interfaceLocale = parsed.locale
      }
      if (body?.notificationSounds !== undefined) {
        const parsed = NotificationSounds.validateNotificationSounds(body.notificationSounds)
        if (!parsed.ok) return Respond.r400(parsed.error)
        next.notificationSounds = /** @type {any} */ (parsed.sounds)
      }
      if (body?.themePreferences !== undefined) {
        const parsed = ThemePreferences.validateThemePreferences(body.themePreferences)
        if (!parsed.ok) return Respond.r400(parsed.error)
        next.themePreferences = /** @type {any} */ (parsed.preferences)
      }
      if (body?.githubProfileEnrichment !== undefined) {
        if (typeof body.githubProfileEnrichment !== 'boolean') {
          return Respond.r400('githubProfileEnrichment must be a boolean')
        }
        next.githubProfileEnrichment = body.githubProfileEnrichment
      }
      if (body?.keyboardShortcutOverrides !== undefined) {
        const parsed = KeyboardShortcuts.validateKeyboardShortcutOverrides(body.keyboardShortcutOverrides)
        if (!parsed.ok) return Respond.r400(parsed.error)
        next.keyboardShortcutOverrides = parsed.overrides
      }
      const grammarPatch = GrammarPreferences.validateGrammarPreferencePatch(body || {})
      if (!grammarPatch.ok) return Respond.r400(grammarPatch.error || 'Invalid grammar preferences')
      if (body?.grammarCheckEnabled !== undefined) next.grammarCheckEnabled = body.grammarCheckEnabled
      if (body?.grammarCheckLanguage !== undefined) next.grammarCheckLanguage = body.grammarCheckLanguage
      if (body?.grammarCheckDisabledRules !== undefined) next.grammarCheckDisabledRules = [...new Set(body.grammarCheckDisabledRules)]
      const unreadBadgePatch = UnreadBadgePreferences.validateUnreadBadgePreferencePatch(body, user, user.domains)
      if (!unreadBadgePatch.ok) {
        return unreadBadgePatch.forbidden
          ? Respond.r403(unreadBadgePatch.error)
          : Respond.r400(unreadBadgePatch.error)
      }
      Object.assign(next, unreadBadgePatch.preferences)
      let externalAccountIds = Array.isArray(user.newMailNotificationExternalAccounts)
        ? user.newMailNotificationExternalAccounts
        : []
      if (body?.newMailNotificationExternalAccounts !== undefined) {
        try {
          const listed = await ExternalAccounts.list(ExternalAccounts.bearerToken(context))
          externalAccountIds = (Array.isArray(listed) ? listed : listed?.accounts || []).map(account => account?.id).filter(Boolean)
        } catch (error) {
          return ExternalAccounts.accountFailure(error)
        }
      }
      const notificationPatch = NewMailNotificationPreferences.validateNewMailNotificationPreferencePatch(body, user, user.domains, externalAccountIds)
      if (!notificationPatch.ok) {
        return notificationPatch.forbidden
          ? Respond.r403(notificationPatch.error)
          : Respond.r400(notificationPatch.error)
      }
      const { newMailNotificationExternalAccounts, ...notificationPreferences } = notificationPatch.preferences
      Object.assign(next, notificationPreferences)
      if (body?.newMailNotificationExternalAccounts !== undefined || Array.isArray(user.newMailNotificationExternalAccounts)) {
        next.newMailNotificationExternalAccounts = newMailNotificationExternalAccounts
      }
      if (body?.mailPreferences !== undefined) {
        const mailPatch = MailPreferences.validateMailPreferencePatch(body.mailPreferences, user)
        if (!mailPatch.ok) return Respond.r400(mailPatch.error)
        next.mailPreferences = mailPatch.preferences
      }

      const updated = await app.users.updatePreferences(docId, next)
      const grammarPreferences = GrammarPreferences.normalizeGrammarPreferences(updated)
      const unreadBadgePreferences = UnreadBadgePreferences.normalizeUnreadBadgePreferences(updated, updated.domains)
      const newMailNotificationPreferences = NewMailNotificationPreferences.normalizeNewMailNotificationPreferencesForAccounts(updated, updated.domains, externalAccountIds)
      return {
        updated: true,
        calendarVisibility: updated.calendarVisibility || 'private',
        mailboxNames: updated.mailboxNames || {},
        categoryMappings: updated.categoryMappings || {},
        remoteImagePolicy: updated.remoteImagePolicy || 'block',
        trustedImageSenders: updated.trustedImageSenders || [],
        interfaceFont: updated.interfaceFont || FontPreferences.DEFAULT_INTERFACE_FONT,
        composerFont: updated.composerFont || FontPreferences.DEFAULT_COMPOSER_FONT,
        interfaceLocale: updated.interfaceLocale || LocalePreferences.DEFAULT_INTERFACE_LOCALE,
        notificationSounds: updated.notificationSounds || NotificationSounds.DEFAULT_NOTIFICATION_SOUNDS,
        themePreferences: updated.themePreferences || ThemePreferences.DEFAULT_THEME_PREFERENCES,
        ...unreadBadgePreferences,
        ...newMailNotificationPreferences,
        githubProfileEnrichment: updated.githubProfileEnrichment === true,
        keyboardShortcutOverrides: KeyboardShortcuts.normalizeKeyboardShortcutOverrides(updated.keyboardShortcutOverrides),
        mailPreferences: MailPreferences.normalizeMailPreferences(updated.mailPreferences, updated),
        ...grammarPreferences,
      }
    })
  }

  /**
   * GET /users/me/preferences
   *
   * Return mutable user preferences. `mailboxNames` contains personal display
   * labels only; mailbox authorization still uses the underlying domains.
   *
   * @param {object} params
   * @param {{ bearer?: { token: string } }} params.context
   * @returns {Promise<unknown>}
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
      const app = await getApp()
      const [, user] = await app.users.findByEmail(auth.email)
      if (!user) return Respond.r404('User not found')
      const grammarPreferences = GrammarPreferences.normalizeGrammarPreferences(user)
      const unreadBadgePreferences = UnreadBadgePreferences.normalizeUnreadBadgePreferences(user, user.domains)
      const newMailNotificationPreferences = NewMailNotificationPreferences.normalizeNewMailNotificationPreferences(user, user.domains)
      return {
        calendarVisibility: /** @type {any} */ (user).calendarVisibility || 'private',
        mailboxNames: /** @type {any} */ (user).mailboxNames || {},
        categoryMappings: /** @type {any} */ (user).categoryMappings || {},
        remoteImagePolicy: /** @type {any} */ (user).remoteImagePolicy || 'block',
        trustedImageSenders: /** @type {any} */ (user).trustedImageSenders || [],
        interfaceFont: /** @type {any} */ (user).interfaceFont || FontPreferences.DEFAULT_INTERFACE_FONT,
        composerFont: /** @type {any} */ (user).composerFont || FontPreferences.DEFAULT_COMPOSER_FONT,
        interfaceLocale: /** @type {any} */ (user).interfaceLocale || LocalePreferences.DEFAULT_INTERFACE_LOCALE,
        notificationSounds: /** @type {any} */ (user).notificationSounds || NotificationSounds.DEFAULT_NOTIFICATION_SOUNDS,
        themePreferences: /** @type {any} */ (user).themePreferences || ThemePreferences.DEFAULT_THEME_PREFERENCES,
        ...unreadBadgePreferences,
        ...newMailNotificationPreferences,
        githubProfileEnrichment: /** @type {any} */ (user).githubProfileEnrichment === true,
        keyboardShortcutOverrides: KeyboardShortcuts.normalizeKeyboardShortcutOverrides(/** @type {any} */ (user).keyboardShortcutOverrides),
        mailPreferences: MailPreferences.normalizeMailPreferences(user.mailPreferences, user),
        ...grammarPreferences,
      }
    })
  }

  /**
   * @param {unknown} value
   * @param {string[]} assignedDomains
   * @returns {{ ok: true, mailboxNames: Record<string, string> } | { ok: false, forbidden?: boolean, error: string }}
   */
  static _normalizeMailboxNames(value, assignedDomains) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return { ok: false, error: 'mailboxNames must be an object' }
    }
    const allowedDomains = new Set(assignedDomains.map(domain => Security.normalizeDomain(domain)).filter(Boolean))
    const seenNames = new Set()
    /** @type {Record<string, string>} */
    const mailboxNames = {}

    for (const [rawDomain, rawName] of Object.entries(value)) {
      const domain = Security.normalizeDomain(rawDomain)
      if (!domain || !allowedDomains.has(domain)) {
        return { ok: false, forbidden: true, error: 'Mailbox domain is not assigned to this user' }
      }
      if (typeof rawName !== 'string') return { ok: false, error: 'Mailbox name must be text' }
      const name = rawName.trim()
      if (!name) continue
      if (Security.hasControlChars(name) || name.length > UsersMePreferencesController.MAX_MAILBOX_NAME_LENGTH) {
        return { ok: false, error: `Mailbox name must be 1 to ${UsersMePreferencesController.MAX_MAILBOX_NAME_LENGTH} printable characters` }
      }
      const identity = name.toLowerCase()
      if (seenNames.has(identity)) return { ok: false, error: 'Mailbox names must be unique' }
      seenNames.add(identity)
      mailboxNames[domain] = name
    }

    return { ok: true, mailboxNames }
  }

  /**
   * A trusted sender is a rendering preference, never an authorization rule.
   * Normalize exact mailboxes and cap the list so one profile cannot grow
   * without bound.
   *
   * @param {unknown} value
   * @returns {{ ok: true, senders: string[] } | { ok: false, error: string }}
   */
  static _normalizeTrustedImageSenders(value) {
    if (!Array.isArray(value)) return { ok: false, error: 'trustedImageSenders must be an array' }
    if (value.length > UsersMePreferencesController.MAX_TRUSTED_IMAGE_SENDERS) {
      return { ok: false, error: `trustedImageSenders may contain at most ${UsersMePreferencesController.MAX_TRUSTED_IMAGE_SENDERS} addresses` }
    }
    const senders = []
    for (const entry of value) {
      if (typeof entry !== 'string') return { ok: false, error: 'Trusted image sender must be an email address' }
      const sender = Security.normalizeEmailAddress(entry)
      if (!sender) return { ok: false, error: 'Trusted image sender must be an email address' }
      if (!senders.includes(sender)) senders.push(sender)
    }
    return { ok: true, senders }
  }
}
