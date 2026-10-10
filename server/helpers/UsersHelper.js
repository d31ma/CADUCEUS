import { Fylo } from '../shared/persistence/Fylo.js'
import { Security } from '../shared/security/Security.js'
import { FontPreferences } from '../shared/users/FontPreferences.js'
import { NotificationSounds } from '../shared/users/NotificationSounds.js'
import { ThemePreferences } from '../shared/users/ThemePreferences.js'
import { LocalePreferences } from '../shared/users/LocalePreferences.js'
import { GrammarPreferences } from '../shared/users/GrammarPreferences.js'
import { CategoryMappingRules } from '../shared/mailbox/CategoryMappingRules.js'
import { CategoryCatalog } from '../shared/mailbox/CategoryCatalog.js'
import { Preferences } from '../shared/mcp/Preferences.js'
import { UnreadBadgePreferences } from '../shared/users/UnreadBadgePreferences.js'
import { NewMailNotificationPreferences } from '../shared/users/NewMailNotificationPreferences.js'
import { KeyboardShortcuts } from '../shared/users/KeyboardShortcuts.js'
import { MailPreferences } from '../shared/users/MailPreferences.js'
import { UsersRecords } from '../shared/users/UsersRecords.js'

@Helper
export default class UsersHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @returns {Promise<Array<User & { docId: string }>>}
   */
  async list() {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.USERS, { $ops: [] }).collect())
    return Object.entries(docs).map(([docId, u]) => ({ docId, ...UsersRecords.normalize(/** @type {User} */ (u)) }))
  }

  /**
   * Look up a user by primary email or alias (case-insensitive).
   *
   * @param {string} email
   * @returns {Promise<[string | null, User | null]>}
   */
  async findByEmail(email) {
    const fylo = await this.fyloHelper.database()
    const normalized = Security.normalizeEmailAddress(email)
    if (!normalized) return [null, null]
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.USERS, { $ops: [{ email: { $eq: normalized } }] }).collect())
    const entry = Object.entries(docs)[0]
    if (entry) return [entry[0], UsersRecords.normalize(/** @type {User} */ (entry[1]))]
    const users = await this.list()
    const aliased = users.find(user => user.aliases?.includes(normalized))
    return aliased ? [aliased.docId, UsersRecords.stripDocId(aliased)] : [null, null]
  }

  /**
   * @param {User} user
   * @returns {Promise<string>}
   */
  async put(user) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.USERS, UsersRecords.normalize(user))
  }

  /**
   * Create or update a user keyed by their normalized primary email.
   *
   * @param {User} user
   * @returns {Promise<string>}
   */
  async upsertByEmail(user) {
    const fylo = await this.fyloHelper.database()
    const normalized = UsersRecords.normalize(user)
    const [docId, existing] = await this.findByEmail(normalized.email)
    if (!docId || !existing) return await this.put(normalized)

    await fylo.patchDoc(Fylo.Collections.USERS, {
      [docId]: UsersRecords.normalize({
        ...existing,
        ...normalized,
        id: existing.id || normalized.id,
        aliases: normalized.aliases?.length ? normalized.aliases : existing.aliases || [],
        calendarVisibility: existing.calendarVisibility || normalized.calendarVisibility,
        mailboxNames: Object.keys(normalized.mailboxNames ?? {}).length
          ? normalized.mailboxNames
          : existing.mailboxNames ?? {},
        categoryMappings: existing.categoryMappings || normalized.categoryMappings,
        categoryCatalog: user.categoryCatalog === undefined
          ? existing.categoryCatalog
          : normalized.categoryCatalog,
        remoteImagePolicy: existing.remoteImagePolicy || normalized.remoteImagePolicy,
        trustedImageSenders: normalized.trustedImageSenders?.length
          ? normalized.trustedImageSenders
          : existing.trustedImageSenders ?? [],
        interfaceFont: existing.interfaceFont || normalized.interfaceFont,
        composerFont: existing.composerFont || normalized.composerFont,
        interfaceLocale: user.interfaceLocale === undefined
          ? existing.interfaceLocale
          : normalized.interfaceLocale,
        notificationSounds: existing.notificationSounds || normalized.notificationSounds,
        themePreferences: existing.themePreferences || normalized.themePreferences,
        githubProfileEnrichment: existing.githubProfileEnrichment ?? normalized.githubProfileEnrichment,
        grammarCheckEnabled: existing.grammarCheckEnabled ?? normalized.grammarCheckEnabled,
        grammarCheckLanguage: existing.grammarCheckLanguage || normalized.grammarCheckLanguage,
        grammarCheckDisabledRules: existing.grammarCheckDisabledRules?.length
          ? existing.grammarCheckDisabledRules
          : normalized.grammarCheckDisabledRules,
        unreadBadgeScope: user.unreadBadgeScope === undefined
          ? existing.unreadBadgeScope
          : normalized.unreadBadgeScope,
        unreadBadgeDomain: user.unreadBadgeDomain === undefined
          ? existing.unreadBadgeDomain
          : normalized.unreadBadgeDomain,
        newMailNotificationDomains: user.newMailNotificationDomains === undefined
          ? existing.newMailNotificationDomains
          : normalized.newMailNotificationDomains,
        newMailNotificationPrivacy: user.newMailNotificationPrivacy === undefined
          ? existing.newMailNotificationPrivacy
          : normalized.newMailNotificationPrivacy,
        newMailNotificationActions: user.newMailNotificationActions === undefined
          ? existing.newMailNotificationActions
          : normalized.newMailNotificationActions,
        keyboardShortcutOverrides: user.keyboardShortcutOverrides === undefined
          ? existing.keyboardShortcutOverrides
          : normalized.keyboardShortcutOverrides,
        mailPreferences: user.mailPreferences === undefined
          ? existing.mailPreferences
          : normalized.mailPreferences,
        mcp: existing.mcp || normalized.mcp,
      }),
    })
    return docId
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    const fylo = await this.fyloHelper.database()
    await fylo.delDoc(Fylo.Collections.USERS, docId)
  }

  /**
   * @param {string} docId
   * @param {string[]} aliases
   * @param {string} [email]
   * @returns {Promise<void>}
   */
  async updateAliases(docId, aliases, email = '') {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.USERS, { [docId]: { aliases: UsersRecords.normalizeAliases(aliases, email) } })
  }

  /**
   * Update display-only user preferences without creating a new user document.
   *
   * @param {string} docId
   * @param {User} user
   * @returns {Promise<User>}
   */
  async updatePreferences(docId, user) {
    const fylo = await this.fyloHelper.database()
    const normalized = UsersRecords.normalize(user)
    await fylo.patchDoc(Fylo.Collections.USERS, {
      [docId]: {
        calendarVisibility: normalized.calendarVisibility,
        mailboxNames: normalized.mailboxNames ?? {},
        categoryMappings: normalized.categoryMappings ?? {},
        categoryCatalog: normalized.categoryCatalog ?? {},
        remoteImagePolicy: normalized.remoteImagePolicy,
        trustedImageSenders: normalized.trustedImageSenders ?? [],
        interfaceFont: normalized.interfaceFont,
        composerFont: normalized.composerFont,
        interfaceLocale: normalized.interfaceLocale,
        notificationSounds: normalized.notificationSounds,
        themePreferences: normalized.themePreferences,
        githubProfileEnrichment: normalized.githubProfileEnrichment,
        grammarCheckEnabled: normalized.grammarCheckEnabled,
        grammarCheckLanguage: normalized.grammarCheckLanguage,
        grammarCheckDisabledRules: normalized.grammarCheckDisabledRules,
        unreadBadgeScope: normalized.unreadBadgeScope,
        unreadBadgeDomain: normalized.unreadBadgeDomain,
        newMailNotificationDomains: normalized.newMailNotificationDomains,
        newMailNotificationPrivacy: normalized.newMailNotificationPrivacy,
        newMailNotificationActions: normalized.newMailNotificationActions,
        keyboardShortcutOverrides: normalized.keyboardShortcutOverrides,
        mailPreferences: normalized.mailPreferences,
        mcp: normalized.mcp,
      },
    })
    return normalized
  }

  /** @param {string} docId @param {import('../types').McpPreferences} preferences */
  async updateMcpPreferences(docId, preferences) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.USERS, { [docId]: { mcp: preferences } })
  }

  /** Persist an already-normalized exact-domain category catalog. */
  async updateCategoryCatalog(docId, catalog) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.USERS, { [docId]: { categoryCatalog: catalog } })
  }

  static normalize = UsersRecords.normalize
}

