import { Security } from '../../../platform/security/Security.js'
import { addCategoryToCatalog, normalizeCategoryCatalog } from './category-catalog.js'
import { normalizeCategoryPath, presentCategory } from './category-path.js'

export { normalizeCategoryPath } from './category-path.js'

export const CATEGORY_ROLES = Object.freeze(['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'])
const CATEGORY_ROLE_SET = new Set(CATEGORY_ROLES)

/**
 * Category-role mappings keep CADUCEUS' stable system roles separate from a
 * mailbox provider's physical folder or label paths. The object is scoped by
 * exact mailbox domain so future IMAP/Gmail/Microsoft adapters can consume the
 * same contract without gaining authority over another tenant.
 */
export class CategoryMappingService {
  /**
   * @param {import('../../users/repositories/UsersRepository.js').UsersRepository} users
   * @param {import('../repositories/EmailsRepository.js').EmailsRepository} emails
   */
  constructor(users, emails) {
    this.users = users
    this.emails = emails
  }

  /** @param {string} ownerEmail */
  async forOwner(ownerEmail) {
    const [, user] = await this.users.findByEmail(ownerEmail)
    if (!user) return null
    return normalizeCategoryMappings(user.categoryMappings, user.domains)
  }

  /**
   * Return physical categories plus their assigned system role. Category
   * discovery is exact-owner even when several users share a hosted domain.
   *
   * @param {string} ownerEmail
   * @param {string[]} allowedDomains
   */
  async listAccounts(ownerEmail, allowedDomains) {
    const [, user] = await this.users.findByEmail(ownerEmail)
    if (!user) return null
    const mappings = normalizeCategoryMappings(user.categoryMappings, user.domains)
    const catalog = normalizeCategoryCatalog(user.categoryCatalog, user.domains)
    const emails = await this.emails.list(allowedDomains, { folder: 'all', ownerEmail })
    return allowedDomains.slice().sort().map(domain => {
      const assigned = mappings[domain] || defaultRoleMapping()
      const owned = emails.filter(email => email.domain === domain)
      const folders = new Set([...CATEGORY_ROLES.map(role => assigned[role]), ...owned
        .filter(email => email.domain === domain)
        .map(email => normalizeCategoryPath(email.folder || 'inbox'))
        .filter(Boolean)])
      const labels = new Set(owned.flatMap(email => Array.isArray(email.labels) ? email.labels : []))
      for (const [path, kind] of Object.entries(catalog[domain] || {})) {
        const target = kind === 'label' ? labels : folders
        target.add(path)
      }
      const roleByPath = invertRoleMapping(assigned)
      return {
        domain,
        mappings: assigned,
        capabilities: { folders: true, labels: true, create: true },
        categories: [
          ...[...folders].map(path => presentCategory(path, 'folder', roleByPath[path])),
          ...[...labels].map(path => presentCategory(path, 'label')),
        ].sort((left, right) => left.kind.localeCompare(right.kind) || left.path.localeCompare(right.path)),
      }
    })
  }

  /** Create one exact-owner category without granting any new mailbox authority. */
  async createCategory(ownerEmail, input) {
    const [docId, user] = await this.users.findByEmail(ownerEmail)
    if (!docId || !user) return { ok: false, notFound: true, error: 'User not found' }
    const result = addCategoryToCatalog(user.categoryCatalog, input, user.domains)
    if (!result.ok) return result
    if (!result.category) return { ok: false, error: 'Category could not be normalized' }
    await this.users.updateCategoryCatalog(docId, result.catalog)
    return { ok: true, category: presentCategory(result.category.path, result.category.kind) }
  }

  /** @param {string} ownerEmail @param {string[]} domains @param {string} requested */
  async folderFilter(ownerEmail, domains, requested) {
    const mappings = await this.forOwner(ownerEmail)
    if (!mappings || !CATEGORY_ROLE_SET.has(String(requested).toLowerCase())) return null
    return Object.fromEntries(domains.map(domain => [domain, mappings[domain]?.[requested] || requested]))
  }

  /** @param {string} ownerEmail @param {string} domain @param {string} requested */
  async pathFor(ownerEmail, domain, requested) {
    const normalized = normalizeCategoryPath(requested)
    if (!normalized || !CATEGORY_ROLE_SET.has(normalized)) return normalized
    const mappings = await this.forOwner(ownerEmail)
    return mappings?.[domain]?.[normalized] || normalized
  }

  /** @param {string} ownerEmail @param {import('../../../types').StoredEmail} email */
  async presentEmail(ownerEmail, email) {
    const mappings = await this.forOwner(ownerEmail)
    return presentEmailWithMappings(email, mappings || {})
  }

  /** @param {string} ownerEmail @param {import('../../../types').StoredEmail[]} emails */
  async presentEmails(ownerEmail, emails) {
    const mappings = await this.forOwner(ownerEmail)
    return emails.map(email => presentEmailWithMappings(email, mappings || {}))
  }
}

/** @returns {Record<string, string>} */
export function defaultRoleMapping() {
  return Object.fromEntries(CATEGORY_ROLES.map(role => [role, role]))
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
export function normalizeCategoryMappings(value, assignedDomains) {
  const source = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, unknown>} */ (value)
    : {}
  const result = {}
  for (const rawDomain of assignedDomains) {
    const domain = Security.normalizeDomain(rawDomain)
    if (!domain) continue
    const supplied = source[domain] && typeof source[domain] === 'object' && !Array.isArray(source[domain])
      ? /** @type {Record<string, unknown>} */ (source[domain])
      : {}
    const mapping = defaultRoleMapping()
    const used = new Set()
    for (const role of CATEGORY_ROLES) {
      const path = normalizeCategoryPath(supplied[role])
      const selected = path && !used.has(path) ? path : uniqueDefaultPath(role, used)
      mapping[role] = selected
      used.add(selected)
    }
    result[domain] = mapping
  }
  return result
}

/** @param {string} role @param {Set<string>} used */
function uniqueDefaultPath(role, used) {
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
export function validateCategoryMappings(value, assignedDomains) {
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
  const mappings = {}
  for (const domain of allowed) {
    const supplied = input[domain]
    if (!supplied || typeof supplied !== 'object' || Array.isArray(supplied)) {
      return { ok: false, error: `Category mappings for ${domain} must include every system role` }
    }
    const record = /** @type {Record<string, unknown>} */ (supplied)
    if (Object.keys(record).some(role => !CATEGORY_ROLE_SET.has(role))) {
      return { ok: false, error: 'Category mappings contain an unknown system role' }
    }
    const mapping = {}
    const used = new Set()
    for (const role of CATEGORY_ROLES) {
      const path = normalizeCategoryPath(record[role])
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
/** @param {Record<string, string>} mapping */
function invertRoleMapping(mapping) {
  return Object.fromEntries(CATEGORY_ROLES.map(role => [mapping[role], role]))
}

/** @param {import('../../../types').StoredEmail} email @param {Record<string, Record<string, string>>} mappings */
export function presentEmailWithMappings(email, mappings) {
  const path = normalizeCategoryPath(email.folder || 'inbox') || 'inbox'
  const role = invertRoleMapping(mappings[email.domain] || defaultRoleMapping())[path]
  return { ...email, folder: path, systemFolder: role || null }
}
