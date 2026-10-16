import { Security } from '../security/Security.js'

export class NewMailNotificationPreferences {
  /** @type {'full'} */
  static DEFAULT_NEW_MAIL_NOTIFICATION_PRIVACY = 'full'

  static DEFAULT_NEW_MAIL_NOTIFICATION_ACTIONS = true

  static #PRIVACY_LEVELS = new Set(['full', 'sender', 'private'])

  static #MAX_NOTIFICATION_DOMAINS = 50

  static #MAX_NOTIFICATION_EXTERNAL_ACCOUNTS = 50

  static #EXTERNAL_ACCOUNT_ID = /^ext_[A-Za-z0-9_-]{20,80}$/

  /** @param {unknown} value */
  static #preferenceObject(value) {
    return value && typeof value === 'object' && !Array.isArray(value)
      ? /** @type {Record<string, unknown>} */ (value)
      : {}
  }

  /** @param {unknown} domains @returns {string[]} */
  static #assignedDomains(domains) {
    if (!Array.isArray(domains)) return []
    return [...new Set(domains
      .map(domain => typeof domain === 'string' ? Security.normalizeDomain(domain) : null)
      .filter(domain => domain !== null))]
  }

  /**
   * Normalize stored notification policy against the current hosted-account set.
   * Missing policy means every assigned account is enabled for compatibility;
   * an explicitly empty list means notifications are disabled for every account.
   *
   * @param {unknown} value
   * @param {unknown} domains
   */
  static normalizeNewMailNotificationPreferences(value, domains) {
  	return NewMailNotificationPreferences.normalizeNewMailNotificationPreferencesForAccounts(value, domains)
  }

  /**
   * Normalize both hosted-domain and connected-account policy. When an exact
   * allowed external-account set is supplied, a missing stored preference means
   * all of those accounts are enabled for backward compatibility.
   *
   * @param {unknown} value
   * @param {unknown} domains
   * @param {unknown} [externalAccountIds]
   */
  static normalizeNewMailNotificationPreferencesForAccounts(value, domains, externalAccountIds) {
    const input = NewMailNotificationPreferences.#preferenceObject(value)
    const allowed = NewMailNotificationPreferences.#assignedDomains(domains)
    const requested = Array.isArray(input.newMailNotificationDomains)
      ? input.newMailNotificationDomains
      : allowed
    /** @type {string[]} */
    const normalizedDomains = []
    for (const entry of requested) {
      if (typeof entry !== 'string') continue
      const domain = Security.normalizeDomain(entry)
      if (domain && allowed.includes(domain) && !normalizedDomains.includes(domain)) normalizedDomains.push(domain)
    }
    const newMailNotificationDomains = normalizedDomains
    const allowedExternal = Array.isArray(externalAccountIds)
      ? [...new Set(externalAccountIds.filter(id => typeof id === 'string' && NewMailNotificationPreferences.#EXTERNAL_ACCOUNT_ID.test(id)))]
      : null
    const requestedExternal = Array.isArray(input.newMailNotificationExternalAccounts)
      ? input.newMailNotificationExternalAccounts
      : (allowedExternal || [])
    const newMailNotificationExternalAccounts = [...new Set(requestedExternal
      .filter(id => typeof id === 'string' && NewMailNotificationPreferences.#EXTERNAL_ACCOUNT_ID.test(id))
      .filter(id => allowedExternal === null || allowedExternal.includes(id)))]
    const newMailNotificationPrivacy = typeof input.newMailNotificationPrivacy === 'string'
      && NewMailNotificationPreferences.#PRIVACY_LEVELS.has(input.newMailNotificationPrivacy)
      ? /** @type {'full'|'sender'|'private'} */ (input.newMailNotificationPrivacy)
      : NewMailNotificationPreferences.DEFAULT_NEW_MAIL_NOTIFICATION_PRIVACY
    const newMailNotificationActions = typeof input.newMailNotificationActions === 'boolean'
      ? input.newMailNotificationActions
      : NewMailNotificationPreferences.DEFAULT_NEW_MAIL_NOTIFICATION_ACTIONS
    return { newMailNotificationDomains, newMailNotificationExternalAccounts, newMailNotificationPrivacy, newMailNotificationActions }
  }

  /**
   * Validate an HTTP preference patch without allowing a caller to name an
   * account outside their authenticated assignment.
   *
   * @param {unknown} patch
   * @param {unknown} current
   * @param {unknown} domains
   * @returns {{ ok: true, preferences: ReturnType<typeof NewMailNotificationPreferences.normalizeNewMailNotificationPreferences> } | { ok: false, forbidden?: boolean, error: string }}
   */
  /** @param {unknown} patch @param {unknown} current @param {unknown} domains @param {string[]} [externalAccountIds] */
  static validateNewMailNotificationPreferencePatch(patch, current, domains, externalAccountIds = []) {
    const body = NewMailNotificationPreferences.#preferenceObject(patch)
    const allowed = NewMailNotificationPreferences.#assignedDomains(domains)
    const allowedExternal = [...new Set(Array.isArray(externalAccountIds)
      ? externalAccountIds.filter(id => typeof id === 'string' && NewMailNotificationPreferences.#EXTERNAL_ACCOUNT_ID.test(id))
      : [])]
    const existing = NewMailNotificationPreferences.normalizeNewMailNotificationPreferencesForAccounts(current, allowed, allowedExternal)
    let nextDomains = existing.newMailNotificationDomains

    if (body.newMailNotificationDomains !== undefined) {
      if (!Array.isArray(body.newMailNotificationDomains)) {
        return { ok: false, error: 'newMailNotificationDomains must be an array' }
      }
      if (body.newMailNotificationDomains.length > NewMailNotificationPreferences.#MAX_NOTIFICATION_DOMAINS) {
        return { ok: false, error: `newMailNotificationDomains may contain at most ${NewMailNotificationPreferences.#MAX_NOTIFICATION_DOMAINS} accounts` }
      }
      nextDomains = []
      for (const entry of body.newMailNotificationDomains) {
        if (typeof entry !== 'string') {
          return { ok: false, error: 'Each new-mail notification account must be a domain' }
        }
        const domain = Security.normalizeDomain(entry)
        if (!domain) return { ok: false, error: 'Each new-mail notification account must be a domain' }
        if (!allowed.includes(domain)) {
          return { ok: false, forbidden: true, error: 'Notification domain is not assigned to this user' }
        }
        if (!nextDomains.includes(domain)) nextDomains.push(domain)
      }
    }

    let nextExternalAccounts = existing.newMailNotificationExternalAccounts
    if (body.newMailNotificationExternalAccounts !== undefined) {
      if (!Array.isArray(body.newMailNotificationExternalAccounts)) {
        return { ok: false, error: 'newMailNotificationExternalAccounts must be an array' }
      }
      if (body.newMailNotificationExternalAccounts.length > NewMailNotificationPreferences.#MAX_NOTIFICATION_EXTERNAL_ACCOUNTS) {
        return { ok: false, error: `newMailNotificationExternalAccounts may contain at most ${NewMailNotificationPreferences.#MAX_NOTIFICATION_EXTERNAL_ACCOUNTS} accounts` }
      }
      nextExternalAccounts = []
      for (const entry of body.newMailNotificationExternalAccounts) {
        if (typeof entry !== 'string' || !NewMailNotificationPreferences.#EXTERNAL_ACCOUNT_ID.test(entry)) {
          return { ok: false, error: 'Each connected new-mail notification account must be an external account id' }
        }
        if (!allowedExternal.includes(entry)) {
          return { ok: false, forbidden: true, error: 'Notification account is not assigned to this user' }
        }
        if (!nextExternalAccounts.includes(entry)) nextExternalAccounts.push(entry)
      }
    }

    let nextPrivacy = existing.newMailNotificationPrivacy
    if (body.newMailNotificationPrivacy !== undefined) {
      if (typeof body.newMailNotificationPrivacy !== 'string' || !NewMailNotificationPreferences.#PRIVACY_LEVELS.has(body.newMailNotificationPrivacy)) {
        return { ok: false, error: 'newMailNotificationPrivacy must be one of: full, sender, private' }
      }
      nextPrivacy = /** @type {'full'|'sender'|'private'} */ (body.newMailNotificationPrivacy)
    }

    let nextActions = existing.newMailNotificationActions
    if (body.newMailNotificationActions !== undefined) {
      if (typeof body.newMailNotificationActions !== 'boolean') {
        return { ok: false, error: 'newMailNotificationActions must be a boolean' }
      }
      nextActions = body.newMailNotificationActions
    }

    return {
      ok: true,
      preferences: {
        newMailNotificationDomains: nextDomains,
        newMailNotificationExternalAccounts: nextExternalAccounts,
        newMailNotificationPrivacy: nextPrivacy,
        newMailNotificationActions: nextActions,
      },
    }
  }

  /** @param {unknown} value @param {unknown} domains @param {unknown} domain */
  static newMailNotificationsEnabledForDomain(value, domains, domain) {
    const normalized = typeof domain === 'string' ? Security.normalizeDomain(domain) : null
    return normalized !== null
      && NewMailNotificationPreferences.normalizeNewMailNotificationPreferences(value, domains).newMailNotificationDomains.includes(normalized)
  }

  /** @param {unknown} value @param {unknown} domains @param {unknown} accountId */
  static newMailNotificationsEnabledForExternalAccount(value, domains, accountId) {
    if (typeof accountId !== 'string' || !NewMailNotificationPreferences.#EXTERNAL_ACCOUNT_ID.test(accountId)) return false
    return NewMailNotificationPreferences.normalizeNewMailNotificationPreferencesForAccounts(value, domains, [accountId])
      .newMailNotificationExternalAccounts.includes(accountId)
  }
}
