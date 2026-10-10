import { Security } from '../../../platform/security/Security.js'
import { normalizeCategoryPath } from './category-path.js'

const CATEGORY_KINDS = new Set(['folder', 'label'])
const MAX_CATEGORIES_PER_DOMAIN = 200

/** Keep an exact-domain, de-duplicated category catalog. */
export function normalizeCategoryCatalog(value, assignedDomains) {
  const source = value && typeof value === 'object' && !Array.isArray(value) ? value : {}
  const catalog = {}
  for (const rawDomain of assignedDomains) {
    const domain = Security.normalizeDomain(rawDomain)
    if (!domain) continue
    const entries = source[domain] && typeof source[domain] === 'object' && !Array.isArray(source[domain])
      ? source[domain]
      : {}
    catalog[domain] = Object.fromEntries(Object.entries(entries).flatMap(([rawPath, rawKind]) => {
      const entry = normalizeCatalogEntry({ path: rawPath, kind: rawKind })
      return entry ? [[entry.path, entry.kind]] : []
    }).sort(([left], [right]) => left.localeCompare(right)).slice(0, MAX_CATEGORIES_PER_DOMAIN))
  }
  return catalog
}

/** Validate and append one user-created folder or label idempotently. */
export function addCategoryToCatalog(catalog, input, assignedDomains) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    return { ok: false, error: 'Category payload required' }
  }
  const domain = Security.normalizeDomain(input.domain)
  const allowed = new Set(assignedDomains.map(value => Security.normalizeDomain(value)).filter(Boolean))
  if (!domain || !allowed.has(domain)) {
    return { ok: false, forbidden: true, error: 'Category domain is not assigned to this user' }
  }
  const entry = normalizeCatalogEntry(input)
  if (!entry) return { ok: false, error: 'kind and a valid category path are required' }
  const normalized = normalizeCategoryCatalog(catalog, assignedDomains)
  const entries = normalized[domain] || {}
  const existingKind = entries[entry.path]
  if (existingKind) {
    if (existingKind !== entry.kind) return { ok: false, error: 'A category path cannot be both a folder and a label' }
    return { ok: true, catalog: normalized, category: entry }
  }
  if (Object.keys(entries).length >= MAX_CATEGORIES_PER_DOMAIN) {
    return { ok: false, error: `A mailbox cannot contain more than ${MAX_CATEGORIES_PER_DOMAIN} custom categories` }
  }
  normalized[domain] = { ...entries, [entry.path]: entry.kind }
  return { ok: true, catalog: normalized, category: entry }
}

/** @param {unknown} value */
function normalizeCatalogEntry(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const entry = /** @type {Record<string, unknown>} */ (value)
  const kind = typeof entry.kind === 'string' ? entry.kind.toLowerCase() : ''
  const path = normalizeCategoryPath(entry.path)
  return CATEGORY_KINDS.has(kind) && path ? { path, kind } : null
}
