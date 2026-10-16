import { Security } from '../security/Security.js'
import { CategoryPath } from './CategoryPath.js'

export class CategoryCatalog {
  static #CATEGORY_KINDS = new Set(['folder', 'label'])

  static #MAX_CATEGORIES_PER_DOMAIN = 200

  /** Keep an exact-domain, de-duplicated category catalog. @param {unknown} value @param {string[]} assignedDomains */
  static normalizeCategoryCatalog(value, assignedDomains) {
    const source = value && typeof value === 'object' && !Array.isArray(value) ? /** @type {Record<string, unknown>} */ (value) : {}
    /** @type {Record<string, Record<string, 'folder' | 'label'>>} */
    const catalog = {}
    for (const rawDomain of assignedDomains) {
      const domain = Security.normalizeDomain(rawDomain)
      if (!domain) continue
      const entries = source[domain] && typeof source[domain] === 'object' && !Array.isArray(source[domain])
        ? /** @type {Record<string, unknown>} */ (source[domain])
        : {}
      catalog[domain] = Object.fromEntries(Object.entries(entries).flatMap(([rawPath, rawKind]) => {
        const entry = CategoryCatalog.#normalizeCatalogEntry({ path: rawPath, kind: rawKind })
        return entry ? [[entry.path, entry.kind]] : []
      }).sort(([left], [right]) => left.localeCompare(right)).slice(0, CategoryCatalog.#MAX_CATEGORIES_PER_DOMAIN))
    }
    return catalog
  }

  /** Validate and append one user-created folder or label idempotently. @param {unknown} catalog @param {{domain?: unknown, path?: unknown, kind?: unknown} | null | undefined} input @param {string[]} assignedDomains */
  static addCategoryToCatalog(catalog, input, assignedDomains) {
    if (!input || typeof input !== 'object' || Array.isArray(input)) {
      return { ok: false, error: 'Category payload required' }
    }
    const domain = Security.normalizeDomain(/** @type {string} */ (input.domain))
    const allowed = new Set(assignedDomains.map(value => Security.normalizeDomain(value)).filter(Boolean))
    if (!domain || !allowed.has(domain)) {
      return { ok: false, forbidden: true, error: 'Category domain is not assigned to this user' }
    }
    const entry = CategoryCatalog.#normalizeCatalogEntry(input)
    if (!entry) return { ok: false, error: 'kind and a valid category path are required' }
    const normalized = CategoryCatalog.normalizeCategoryCatalog(catalog, assignedDomains)
    const entries = normalized[domain] || {}
    const existingKind = entries[entry.path]
    if (existingKind) {
      if (existingKind !== entry.kind) return { ok: false, error: 'A category path cannot be both a folder and a label' }
      return { ok: true, catalog: normalized, category: entry }
    }
    if (Object.keys(entries).length >= CategoryCatalog.#MAX_CATEGORIES_PER_DOMAIN) {
      return { ok: false, error: `A mailbox cannot contain more than ${CategoryCatalog.#MAX_CATEGORIES_PER_DOMAIN} custom categories` }
    }
    normalized[domain] = { ...entries, [entry.path]: entry.kind }
    return { ok: true, catalog: normalized, category: entry }
  }

  /** @param {unknown} value @returns {{path: string, kind: 'folder' | 'label'} | null} */
  static #normalizeCatalogEntry(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return null
    const entry = /** @type {Record<string, unknown>} */ (value)
    const kind = typeof entry.kind === 'string' ? entry.kind.toLowerCase() : ''
    const path = CategoryPath.normalizeCategoryPath(entry.path)
    return CategoryCatalog.#CATEGORY_KINDS.has(kind) && path ? /** @type {{path: string, kind: 'folder' | 'label'}} */ ({ path, kind }) : null
  }
}
