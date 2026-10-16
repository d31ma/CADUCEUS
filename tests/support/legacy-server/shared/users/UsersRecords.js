import { randomUUID } from 'node:crypto'
import { Fylo } from '../persistence/Fylo.js'
import { Security } from '../security/Security.js'
import { FontPreferences } from './FontPreferences.js'
import { NotificationSounds } from './NotificationSounds.js'
import { ThemePreferences } from './ThemePreferences.js'
import { LocalePreferences } from './LocalePreferences.js'
import { GrammarPreferences } from './GrammarPreferences.js'
import { CategoryMappingRules } from '../mailbox/CategoryMappingRules.js'
import { CategoryCatalog } from '../mailbox/CategoryCatalog.js'
import { Preferences } from '../mcp/Preferences.js'
import { UnreadBadgePreferences } from './UnreadBadgePreferences.js'
import { NewMailNotificationPreferences } from './NewMailNotificationPreferences.js'
import { KeyboardShortcuts } from './KeyboardShortcuts.js'
import { MailPreferences } from './MailPreferences.js'

export class UsersRecords {
  /** @import { User } from '../../types' */

  /**
   * Allowed calendar-visibility values. Stored on the user record (not a
   * separate collection) so the team-events fan-out doesn't need an extra
   * round-trip for the privacy check. Defaults to `private` so a user's events
   * never leak to teammates before they opt in.
   */
  static VALID_CAL_VIS = new Set(['private', 'busy', 'full'])

  static VALID_REMOTE_IMAGE_POLICIES = new Set(['block', 'allow'])

  static MAX_MAILBOX_NAME_LENGTH = 64

  static MAX_TRUSTED_IMAGE_SENDERS = 500

  /** @param {User} user @returns {User} */
  static normalize(user) {
    const { phones: _legacyPhones, ...profile } = /** @type {User & { phones?: string[] }} */ (user)
    const email = Security.normalizeEmailAddress(user.email) ?? user.email.trim().toLowerCase()
    const aliases = UsersRecords.normalizeAliases(user.aliases ?? [], email)
    const domains = UsersRecords.dedupe(user.domains.map(domain => /** @type {string} */ (Security.normalizeDomain(domain))).filter(Boolean))
    const calendarVisibility = UsersRecords.VALID_CAL_VIS.has(/** @type {any} */ (user.calendarVisibility))
      ? /** @type {any} */ (user.calendarVisibility)
      : 'private'
    const mailboxNames = UsersRecords.normalizeMailboxNames(user.mailboxNames ?? {}, domains)
    const categoryMappings = CategoryMappingRules.normalizeCategoryMappings(user.categoryMappings, domains)
    const categoryCatalog = CategoryCatalog.normalizeCategoryCatalog(user.categoryCatalog, domains)
    const remoteImagePolicy = UsersRecords.VALID_REMOTE_IMAGE_POLICIES.has(/** @type {any} */ (user.remoteImagePolicy))
      ? /** @type {'block'|'allow'} */ (user.remoteImagePolicy)
      : 'block'
    const trustedImageSenders = UsersRecords.normalizeTrustedImageSenders(user.trustedImageSenders ?? [])
    const interfaceFont = FontPreferences.normalizeInterfaceFont(user.interfaceFont)
    const composerFont = FontPreferences.normalizeComposerFont(user.composerFont)
    const interfaceLocale = LocalePreferences.normalizeInterfaceLocale(user.interfaceLocale)
    const notificationSounds = NotificationSounds.normalizeNotificationSounds(user.notificationSounds)
    const themePreferences = ThemePreferences.normalizeThemePreferences(user.themePreferences)
    const githubProfileEnrichment = user.githubProfileEnrichment === true
    const grammarPreferences = GrammarPreferences.normalizeGrammarPreferences(user)
    const unreadBadgePreferences = UnreadBadgePreferences.normalizeUnreadBadgePreferences(user, domains)
    const newMailNotificationPreferences = NewMailNotificationPreferences.normalizeNewMailNotificationPreferences(user, domains)
    const keyboardShortcutOverrides = KeyboardShortcuts.normalizeKeyboardShortcutOverrides(user.keyboardShortcutOverrides)
    const mailPreferences = MailPreferences.normalizeMailPreferences(user.mailPreferences, { email, aliases, domains })
    const mcp = Preferences.normalizeMcpPreferences(user.mcp, domains)
    return {
      ...profile,
      id: user.id || randomUUID(),
      email,
      aliases,
      domains,
      calendarVisibility,
      mailboxNames,
      categoryMappings,
      categoryCatalog,
      remoteImagePolicy,
      trustedImageSenders,
      interfaceFont,
      composerFont,
      interfaceLocale,
      notificationSounds: /** @type {any} */ (notificationSounds),
      themePreferences: /** @type {any} */ (themePreferences),
      githubProfileEnrichment,
      ...grammarPreferences,
      ...unreadBadgePreferences,
      ...newMailNotificationPreferences,
      keyboardShortcutOverrides,
      mailPreferences,
      mcp,
    }
  }

  /** @param {string[]} aliases @param {string} email @returns {string[]} */
  static normalizeAliases(aliases, email) {
    return UsersRecords.dedupe(
      aliases
        .map(alias => /** @type {string} */ (Security.normalizeEmailAddress(alias)))
        .filter(Boolean)
        .filter(alias => !email || alias !== email),
    )
  }

  /**
   * Keep only safe display labels for domains that remain assigned to the user.
   *
   * @param {Record<string, string>} mailboxNames
   * @param {string[]} domains
   * @returns {Record<string, string>}
   */
  static normalizeMailboxNames(mailboxNames, domains) {
    if (!mailboxNames || typeof mailboxNames !== 'object' || Array.isArray(mailboxNames)) return {}
    const allowedDomains = new Set(domains)
    /** @type {Record<string, string>} */
    const normalized = {}
    for (const [rawDomain, rawName] of Object.entries(mailboxNames)) {
      const domain = Security.normalizeDomain(rawDomain)
      const name = typeof rawName === 'string' ? rawName.trim() : ''
      if (!domain || !allowedDomains.has(domain) || !name) continue
      if (name.length > UsersRecords.MAX_MAILBOX_NAME_LENGTH || /[\0-\x1f\x7f]/.test(name)) continue
      normalized[domain] = name
    }
    return normalized
  }

  /** @param {string[]} senders @returns {string[]} */
  static normalizeTrustedImageSenders(senders) {
    if (!Array.isArray(senders)) return []
    return UsersRecords.dedupe(senders
      .map(sender => /** @type {string} */ (Security.normalizeEmailAddress(sender)))
      .filter(Boolean))
      .slice(0, UsersRecords.MAX_TRUSTED_IMAGE_SENDERS)
  }

  /** @param {User & { docId: string }} user */
  static stripDocId(user) {
    const { docId, ...rest } = user
    void docId
    return rest
  }

  /** @param {string[]} values @returns {string[]} */
  static dedupe(values) {
    return [...new Set(values)]
  }
}
