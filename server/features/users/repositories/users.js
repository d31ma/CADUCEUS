import { randomUUID } from 'node:crypto'
import { Collections, collect } from '../../../platform/persistence/fylo.js'
import { Security } from '../../../platform/security/security.js'
import {
  normalizeComposerFont,
  normalizeInterfaceFont,
} from '../services/font-preferences.js'
import { normalizeNotificationSounds } from '../services/notification-sounds.js'
import { normalizeThemePreferences } from '../services/theme-preferences.js'
import { normalizeInterfaceLocale } from '../services/locale-preferences.js'
import { normalizeGrammarPreferences } from '../services/grammar-preferences.js'
import { normalizeCategoryMappings } from '../../mailbox/services/category-mappings.js'
import { normalizeCategoryCatalog } from '../../mailbox/services/category-catalog.js'
import { normalizeMcpPreferences } from '../../mcp/services/preferences.js'
import { normalizeUnreadBadgePreferences } from '../services/unread-badge-preferences.js'
import { normalizeNewMailNotificationPreferences } from '../services/new-mail-notification-preferences.js'
import { normalizeKeyboardShortcutOverrides } from '../services/keyboard-shortcuts.js'
import { normalizeMailPreferences } from '../services/mail-preferences.js'

/**
 * Allowed calendar-visibility values. Stored on the user record (not a
 * separate collection) so the team-events fan-out doesn't need an extra
 * round-trip for the privacy check. Defaults to `private` so a user's events
 * never leak to teammates before they opt in.
 */
const VALID_CAL_VIS = new Set(['private', 'busy', 'full'])
const VALID_REMOTE_IMAGE_POLICIES = new Set(['block', 'allow'])
const MAX_MAILBOX_NAME_LENGTH = 64
const MAX_TRUSTED_IMAGE_SENDERS = 500

export class UsersRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @returns {Promise<Array<User & { docId: string }>>}
   */
  async list() {
    const docs = await collect(this.fylo.findDocs(Collections.USERS, { $ops: [] }).collect())
    return Object.entries(docs).map(([docId, u]) => ({ docId, ...normalize(/** @type {User} */ (u)) }))
  }

  /**
   * Look up a user by primary email or alias (case-insensitive).
   *
   * @param {string} email
   * @returns {Promise<[string | null, User | null]>}
   */
  async findByEmail(email) {
    const normalized = Security.normalizeEmailAddress(email)
    if (!normalized) return [null, null]
    const docs = await collect(this.fylo.findDocs(Collections.USERS, { $ops: [{ email: { $eq: normalized } }] }).collect())
    const entry = Object.entries(docs)[0]
    if (entry) return [entry[0], normalize(/** @type {User} */ (entry[1]))]
    const users = await this.list()
    const aliased = users.find(user => user.aliases?.includes(normalized))
    return aliased ? [aliased.docId, stripDocId(aliased)] : [null, null]
  }

  /**
   * @param {User} user
   * @returns {Promise<string>}
   */
  async put(user) {
    return await this.fylo.putData(Collections.USERS, normalize(user))
  }

  /**
   * Create or update a user keyed by their normalized primary email.
   *
   * @param {User} user
   * @returns {Promise<string>}
   */
  async upsertByEmail(user) {
    const normalized = normalize(user)
    const [docId, existing] = await this.findByEmail(normalized.email)
    if (!docId || !existing) return await this.put(normalized)

    await this.fylo.patchDoc(Collections.USERS, {
      [docId]: normalize({
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
    await this.fylo.delDoc(Collections.USERS, docId)
  }

  /**
   * @param {string} docId
   * @param {string[]} aliases
   * @param {string} [email]
   * @returns {Promise<void>}
   */
  async updateAliases(docId, aliases, email = '') {
    await this.fylo.patchDoc(Collections.USERS, { [docId]: { aliases: normalizeAliases(aliases, email) } })
  }

  /**
   * Update display-only user preferences without creating a new user document.
   *
   * @param {string} docId
   * @param {User} user
   * @returns {Promise<User>}
   */
  async updatePreferences(docId, user) {
    const normalized = normalize(user)
    await this.fylo.patchDoc(Collections.USERS, {
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

  /** @param {string} docId @param {import('../../../types').McpPreferences} preferences */
  async updateMcpPreferences(docId, preferences) {
    await this.fylo.patchDoc(Collections.USERS, { [docId]: { mcp: preferences } })
  }

  /** Persist an already-normalized exact-domain category catalog. */
  async updateCategoryCatalog(docId, catalog) {
    await this.fylo.patchDoc(Collections.USERS, { [docId]: { categoryCatalog: catalog } })
  }

  static normalize = normalize
}

/** @param {User} user @returns {User} */
function normalize(user) {
  const { phones: _legacyPhones, ...profile } = /** @type {User & { phones?: string[] }} */ (user)
  const email = Security.normalizeEmailAddress(user.email) ?? user.email.trim().toLowerCase()
  const aliases = normalizeAliases(user.aliases ?? [], email)
  const domains = dedupe(user.domains.map(domain => /** @type {string} */ (Security.normalizeDomain(domain))).filter(Boolean))
  const calendarVisibility = VALID_CAL_VIS.has(/** @type {any} */ (user.calendarVisibility))
    ? /** @type {any} */ (user.calendarVisibility)
    : 'private'
  const mailboxNames = normalizeMailboxNames(user.mailboxNames ?? {}, domains)
  const categoryMappings = normalizeCategoryMappings(user.categoryMappings, domains)
  const categoryCatalog = normalizeCategoryCatalog(user.categoryCatalog, domains)
  const remoteImagePolicy = VALID_REMOTE_IMAGE_POLICIES.has(/** @type {any} */ (user.remoteImagePolicy))
    ? /** @type {'block'|'allow'} */ (user.remoteImagePolicy)
    : 'block'
  const trustedImageSenders = normalizeTrustedImageSenders(user.trustedImageSenders ?? [])
  const interfaceFont = normalizeInterfaceFont(user.interfaceFont)
  const composerFont = normalizeComposerFont(user.composerFont)
  const interfaceLocale = normalizeInterfaceLocale(user.interfaceLocale)
  const notificationSounds = normalizeNotificationSounds(user.notificationSounds)
  const themePreferences = normalizeThemePreferences(user.themePreferences)
  const githubProfileEnrichment = user.githubProfileEnrichment === true
  const grammarPreferences = normalizeGrammarPreferences(user)
  const unreadBadgePreferences = normalizeUnreadBadgePreferences(user, domains)
  const newMailNotificationPreferences = normalizeNewMailNotificationPreferences(user, domains)
  const keyboardShortcutOverrides = normalizeKeyboardShortcutOverrides(user.keyboardShortcutOverrides)
  const mailPreferences = normalizeMailPreferences(user.mailPreferences, { email, aliases, domains })
  const mcp = normalizeMcpPreferences(user.mcp, domains)
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
function normalizeAliases(aliases, email) {
  return dedupe(
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
function normalizeMailboxNames(mailboxNames, domains) {
  if (!mailboxNames || typeof mailboxNames !== 'object' || Array.isArray(mailboxNames)) return {}
  const allowedDomains = new Set(domains)
  /** @type {Record<string, string>} */
  const normalized = {}
  for (const [rawDomain, rawName] of Object.entries(mailboxNames)) {
    const domain = Security.normalizeDomain(rawDomain)
    const name = typeof rawName === 'string' ? rawName.trim() : ''
    if (!domain || !allowedDomains.has(domain) || !name) continue
    if (name.length > MAX_MAILBOX_NAME_LENGTH || /[\0-\x1f\x7f]/.test(name)) continue
    normalized[domain] = name
  }
  return normalized
}

/** @param {string[]} senders @returns {string[]} */
function normalizeTrustedImageSenders(senders) {
  if (!Array.isArray(senders)) return []
  return dedupe(senders
    .map(sender => /** @type {string} */ (Security.normalizeEmailAddress(sender)))
    .filter(Boolean))
    .slice(0, MAX_TRUSTED_IMAGE_SENDERS)
}

/** @param {User & { docId: string }} user */
function stripDocId(user) {
  const { docId, ...rest } = user
  void docId
  return rest
}

/** @param {string[]} values @returns {string[]} */
function dedupe(values) {
  return [...new Set(values)]
}
