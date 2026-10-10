import { Security } from '../../../shared/security/Security.js'
import { CategoryCatalog } from '../../../shared/mailbox/CategoryCatalog.js'
import { CategoryPath } from '../../../shared/mailbox/CategoryPath.js'
import { CategoryMappingRules } from '../../../shared/mailbox/CategoryMappingRules.js'

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
    return CategoryMappingRules.normalizeCategoryMappings(user.categoryMappings, user.domains)
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
    const mappings = CategoryMappingRules.normalizeCategoryMappings(user.categoryMappings, user.domains)
    const catalog = CategoryCatalog.normalizeCategoryCatalog(user.categoryCatalog, user.domains)
    const emails = await this.emails.list(allowedDomains, { folder: 'all', ownerEmail })
    return allowedDomains.slice().sort().map(domain => {
      const assigned = mappings[domain] || CategoryMappingRules.defaultRoleMapping()
      const owned = emails.filter(email => email.domain === domain)
      const folders = new Set([...CategoryMappingRules.CATEGORY_ROLES.map(role => assigned[role]), ...owned
        .filter(email => email.domain === domain)
        .map(email => CategoryPath.normalizeCategoryPath(email.folder || 'inbox'))
        .filter(Boolean)])
      const labels = new Set(owned.flatMap(email => Array.isArray(email.labels) ? email.labels : []))
      for (const [path, kind] of Object.entries(catalog[domain] || {})) {
        const target = kind === 'label' ? labels : folders
        target.add(path)
      }
      const roleByPath = CategoryMappingRules.invertRoleMapping(assigned)
      return {
        domain,
        mappings: assigned,
        capabilities: { folders: true, labels: true, create: true },
        categories: [
          ...[...folders].map(path => CategoryPath.presentCategory(path, 'folder', roleByPath[path])),
          ...[...labels].map(path => CategoryPath.presentCategory(path, 'label')),
        ].sort((left, right) => left.kind.localeCompare(right.kind) || left.path.localeCompare(right.path)),
      }
    })
  }

  /** Create one exact-owner category without granting any new mailbox authority. */
  async createCategory(ownerEmail, input) {
    const [docId, user] = await this.users.findByEmail(ownerEmail)
    if (!docId || !user) return { ok: false, notFound: true, error: 'User not found' }
    const result = CategoryCatalog.addCategoryToCatalog(user.categoryCatalog, input, user.domains)
    if (!result.ok) return result
    if (!result.category) return { ok: false, error: 'Category could not be normalized' }
    await this.users.updateCategoryCatalog(docId, result.catalog)
    return { ok: true, category: CategoryPath.presentCategory(result.category.path, result.category.kind) }
  }

  /** @param {string} ownerEmail @param {string[]} domains @param {string} requested */
  async folderFilter(ownerEmail, domains, requested) {
    const mappings = await this.forOwner(ownerEmail)
    if (!mappings || !CategoryMappingRules.CATEGORY_ROLE_SET.has(String(requested).toLowerCase())) return null
    return Object.fromEntries(domains.map(domain => [domain, mappings[domain]?.[requested] || requested]))
  }

  /** @param {string} ownerEmail @param {string} domain @param {string} requested */
  async pathFor(ownerEmail, domain, requested) {
    const normalized = CategoryPath.normalizeCategoryPath(requested)
    if (!normalized || !CategoryMappingRules.CATEGORY_ROLE_SET.has(normalized)) return normalized
    const mappings = await this.forOwner(ownerEmail)
    return mappings?.[domain]?.[normalized] || normalized
  }

  /** @param {string} ownerEmail @param {import('../../../types').StoredEmail} email */
  async presentEmail(ownerEmail, email) {
    const mappings = await this.forOwner(ownerEmail)
    return CategoryMappingRules.presentEmailWithMappings(email, mappings || {})
  }

  /** @param {string} ownerEmail @param {import('../../../types').StoredEmail[]} emails */
  async presentEmails(ownerEmail, emails) {
    const mappings = await this.forOwner(ownerEmail)
    return emails.map(email => CategoryMappingRules.presentEmailWithMappings(email, mappings || {}))
  }
}

