import { Security } from '../security/Security.js'
import { CategoryCatalog } from './CategoryCatalog.js'
import { CategoryPath } from './CategoryPath.js'

export class CategoryMappingRules {
  static CATEGORY_ROLES = Object.freeze(['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'])

  static CATEGORY_ROLE_SET = new Set(CategoryMappingRules.CATEGORY_ROLES)

  /** @returns {Record<string, string>} */
  static defaultRoleMapping() {
    return Object.fromEntries(CategoryMappingRules.CATEGORY_ROLES.map(role => [role, role]))
  }

  /**
   * Normalize persisted data defensively. Missing domains and roles receive
   * native CADUCEUS defaults; unknown domains and ambiguous duplicate paths are
   * discarded so a damaged preference cannot widen mailbox access.
   *
   * @param {unknown} value
   * @param {string[]} assignedDomains
   * @returns {Record<string, Record<string, string>>}
   */
  static normalizeCategoryMappings(value, assignedDomains) {
    const source = value && typeof value === 'object' && !Array.isArray(value)
      ? /** @type {Record<string, unknown>} */ (value)
      : {}
    /** @type {Record<string, Record<string, string>>} */
    const result = {}
    for (const rawDomain of assignedDomains) {
      const domain = Security.normalizeDomain(rawDomain)
      if (!domain) continue
      const supplied = source[domain] && typeof source[domain] === 'object' && !Array.isArray(source[domain])
        ? /** @type {Record<string, unknown>} */ (source[domain])
        : {}
      const mapping = CategoryMappingRules.defaultRoleMapping()
      const used = new Set()
      for (const role of CategoryMappingRules.CATEGORY_ROLES) {
        const path = CategoryPath.normalizeCategoryPath(supplied[role])
        const selected = path && !used.has(path) ? path : CategoryMappingRules.uniqueDefaultPath(role, used)
        mapping[role] = selected
        used.add(selected)
      }
      result[domain] = mapping
    }
    return result
  }

  /** @param {string} role @param {Set<string>} used */
  static uniqueDefaultPath(role, used) {
    if (!used.has(role)) return role
    const systemPath = `system/${role}`
    if (!used.has(systemPath)) return systemPath
    let suffix = 2
    while (used.has(`${systemPath}-${suffix}`)) suffix += 1
    return `${systemPath}-${suffix}`
  }

  /**
   * Strictly validate a complete settings patch. Every assigned domain and role
   * must be present, paths must be safe bounded provider identifiers, and one
   * physical path cannot carry two system roles.
   *
   * @param {unknown} value
   * @param {string[]} assignedDomains
   * @returns {{ ok: true, mappings: Record<string, Record<string, string>> } | { ok: false, forbidden?: boolean, error: string }}
   */
  static validateCategoryMappings(value, assignedDomains) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return { ok: false, error: 'categoryMappings must be an object' }
    }
    /** @type {Set<string>} */
    const allowed = new Set()
    for (const rawDomain of assignedDomains) {
      const domain = Security.normalizeDomain(rawDomain)
      if (domain) allowed.add(domain)
    }
    const input = /** @type {Record<string, unknown>} */ (value)
    for (const rawDomain of Object.keys(input)) {
      const domain = Security.normalizeDomain(rawDomain)
      if (!domain || !allowed.has(domain)) {
        return { ok: false, forbidden: true, error: 'Category mapping domain is not assigned to this user' }
      }
    }
    /** @type {Record<string, Record<string, string>>} */
    const mappings = {}
    for (const domain of allowed) {
      const supplied = input[domain]
      if (!supplied || typeof supplied !== 'object' || Array.isArray(supplied)) {
        return { ok: false, error: `Category mappings for ${domain} must include every system role` }
      }
      const record = /** @type {Record<string, unknown>} */ (supplied)
      if (Object.keys(record).some(role => !CategoryMappingRules.CATEGORY_ROLE_SET.has(role))) {
        return { ok: false, error: 'Category mappings contain an unknown system role' }
      }
      /** @type {Record<string, string>} */
      const mapping = {}
      const used = new Set()
      for (const role of CategoryMappingRules.CATEGORY_ROLES) {
        const path = CategoryPath.normalizeCategoryPath(record[role])
        if (!path) return { ok: false, error: `${role} must map to a valid folder path` }
        if (used.has(path)) return { ok: false, error: 'Each system role must map to a different folder path' }
        used.add(path)
        mapping[role] = path
      }
      mappings[domain] = mapping
    }
    return { ok: true, mappings }
  }

  /** @param {unknown} value @returns {string} */
  /** @param {Record<string, string>} mapping @returns {Record<string, string>} */
  static invertRoleMapping(mapping) {
    return Object.fromEntries(CategoryMappingRules.CATEGORY_ROLES.map(role => [mapping[role], role]))
  }

  /** @param {import('../../types').StoredEmail} email @param {Record<string, Record<string, string>>} mappings */
  static presentEmailWithMappings(email, mappings) {
    const path = CategoryPath.normalizeCategoryPath(email.folder || 'inbox') || 'inbox'
    const role = CategoryMappingRules.invertRoleMapping(mappings[email.domain] || CategoryMappingRules.defaultRoleMapping())[path]
    return { ...email, folder: path, systemFolder: role || null }
  }
}
