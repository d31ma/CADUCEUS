import { Security } from '../security/Security.js'

export class UnreadBadgePreferences {
  /** @type {'all'} */
  static DEFAULT_UNREAD_BADGE_SCOPE = 'all'

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
      .filter((domain) => domain !== null))]
  }

  /**
   * Normalize stored badge preferences against the user's current account set.
   * Removed domains fail safely back to the aggregate scope.
   *
   * @param {unknown} value
   * @param {unknown} domains
   * @returns {{ unreadBadgeScope: 'all'|'domain', unreadBadgeDomain: string }}
   */
  static normalizeUnreadBadgePreferences(value, domains) {
    const input = UnreadBadgePreferences.#preferenceObject(value)
    const allowed = UnreadBadgePreferences.#assignedDomains(domains)
    const requestedDomain = typeof input.unreadBadgeDomain === 'string'
      ? Security.normalizeDomain(input.unreadBadgeDomain)
      : null
    const unreadBadgeDomain = requestedDomain && allowed.includes(requestedDomain)
      ? requestedDomain
      : allowed[0] || ''
    const unreadBadgeScope = input.unreadBadgeScope === 'domain' && requestedDomain === unreadBadgeDomain
      ? 'domain'
      : UnreadBadgePreferences.DEFAULT_UNREAD_BADGE_SCOPE
    return { unreadBadgeScope, unreadBadgeDomain }
  }

  /**
   * Validate a partial HTTP preference update without trusting caller-controlled
   * domain scope.
   *
   * @param {unknown} patch
   * @param {unknown} current
   * @param {unknown} domains
   * @returns {{ ok: true, preferences: { unreadBadgeScope: 'all'|'domain', unreadBadgeDomain: string } } | { ok: false, forbidden?: boolean, error: string }}
   */
  static validateUnreadBadgePreferencePatch(patch, current, domains) {
    const body = UnreadBadgePreferences.#preferenceObject(patch)
    const allowed = UnreadBadgePreferences.#assignedDomains(domains)
    const existing = UnreadBadgePreferences.normalizeUnreadBadgePreferences(current, allowed)

    if (body.unreadBadgeScope !== undefined
      && (typeof body.unreadBadgeScope !== 'string' || !['all', 'domain'].includes(body.unreadBadgeScope))) {
      return { ok: false, error: 'unreadBadgeScope must be one of: all, domain' }
    }

    let unreadBadgeDomain = existing.unreadBadgeDomain
    if (body.unreadBadgeDomain !== undefined) {
      if (typeof body.unreadBadgeDomain !== 'string') {
        return { ok: false, error: 'unreadBadgeDomain must be an assigned domain' }
      }
      const domain = Security.normalizeDomain(body.unreadBadgeDomain)
      if (!domain) return { ok: false, error: 'unreadBadgeDomain must be an assigned domain' }
      if (!allowed.includes(domain)) {
        return { ok: false, forbidden: true, error: 'Unread badge domain is not assigned to this user' }
      }
      unreadBadgeDomain = domain
    }

    const unreadBadgeScope = body.unreadBadgeScope === undefined
      ? existing.unreadBadgeScope
      : /** @type {'all'|'domain'} */ (body.unreadBadgeScope)
    if (unreadBadgeScope === 'domain' && !unreadBadgeDomain) {
      return { ok: false, error: 'unreadBadgeDomain is required for domain scope' }
    }
    return { ok: true, preferences: { unreadBadgeScope, unreadBadgeDomain } }
  }

  /**
   * Resolve the repository domain scope used for a background unread query.
   *
   * @param {unknown} value
   * @param {unknown} domains
   * @param {unknown} fallbackDomain
   */
  static unreadBadgeDomains(value, domains, fallbackDomain = '') {
    const allowed = UnreadBadgePreferences.#assignedDomains(domains)
    const fallback = typeof fallbackDomain === 'string' ? Security.normalizeDomain(fallbackDomain) : null
    const available = allowed.length ? allowed : [fallback].filter((domain) => domain !== null)
    const preferences = UnreadBadgePreferences.normalizeUnreadBadgePreferences(value, available)
    return preferences.unreadBadgeScope === 'domain' && preferences.unreadBadgeDomain
      ? [preferences.unreadBadgeDomain]
      : available
  }
}
