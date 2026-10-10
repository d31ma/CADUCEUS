import { Security } from '../security/Security.js'
import { normalizeCategoryPath } from '../../features/mailbox/services/CategoryMappingService.js'

export const MCP_ACCESS_LEVELS = Object.freeze(['read-only', 'read-write', 'read-write-send'])
const ACCESS_LEVEL_SET = new Set(MCP_ACCESS_LEVELS)
const MAX_DOMAINS = 100
const MAX_EXCLUDED_FOLDERS_PER_DOMAIN = 100
const MAX_EXCLUDED_FOLDERS = 500
const FIELDS = new Set(['enabled', 'accessLevel', 'allowedDomains', 'excludedFolders'])

/**
 * Convert persisted MCP preferences into a complete, least-surprising view.
 * A pre-MCP user starts disabled and scoped to every assigned domain, matching
 * MailSpring's "all accounts" default without exposing anything until opt-in.
 *
 * @param {unknown} source
 * @param {string[]} assignedDomains
 */
export function normalizeMcpPreferences(source, assignedDomains) {
  const assigned = normalizeDomains(assignedDomains).slice(0, MAX_DOMAINS)
  const value = source && typeof source === 'object' && !Array.isArray(source)
    ? /** @type {Record<string, any>} */ (source)
    : {}
  const configured = Array.isArray(value.allowedDomains)
    ? normalizeDomains(value.allowedDomains).filter(domain => assigned.includes(domain))
    : assigned
  const allowedDomains = configured.slice(0, MAX_DOMAINS)
  return /** @type {import('../../types').McpPreferences} */ ({
    enabled: value.enabled === true,
    accessLevel: ACCESS_LEVEL_SET.has(value.accessLevel) ? value.accessLevel : 'read-only',
    allowedDomains,
    excludedFolders: normalizeExcludedFolders(value.excludedFolders, allowedDomains),
  })
}

/**
 * Strictly validate a complete MCP preference object. Unknown fields and
 * out-of-scope domains fail closed; exclusions may only name safe physical
 * folder paths inside an enabled account.
 *
 * @param {unknown} input
 * @param {string[]} assignedDomains
 * @returns {{ok:true, preferences:import('../../types').McpPreferences}|{ok:false, forbidden?:boolean, error:string}}
 */
export function validateMcpPreferences(input, assignedDomains) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    return { ok: false, error: 'MCP configuration must be an object' }
  }
  const value = /** @type {Record<string, unknown>} */ (input)
  if (Object.keys(value).some(field => !FIELDS.has(field))) {
    return { ok: false, error: 'MCP configuration contains an unknown field' }
  }
  if (typeof value.enabled !== 'boolean') return { ok: false, error: 'enabled must be a boolean' }
  if (typeof value.accessLevel !== 'string' || !ACCESS_LEVEL_SET.has(value.accessLevel)) {
    return { ok: false, error: `accessLevel must be one of: ${MCP_ACCESS_LEVELS.join(', ')}` }
  }
  if (!Array.isArray(value.allowedDomains) || value.allowedDomains.length > MAX_DOMAINS) {
    return { ok: false, error: `allowedDomains must contain at most ${MAX_DOMAINS} domains` }
  }
  const assigned = new Set(normalizeDomains(assignedDomains))
  const allowedDomains = []
  for (const raw of value.allowedDomains) {
    if (typeof raw !== 'string') return { ok: false, error: 'Each allowed domain must be text' }
    const domain = Security.normalizeDomain(raw)
    if (!domain || !assigned.has(domain)) {
      return { ok: false, forbidden: true, error: 'MCP domain is not assigned to this user' }
    }
    if (!allowedDomains.includes(domain)) allowedDomains.push(domain)
  }
  if (value.enabled && allowedDomains.length === 0) {
    return { ok: false, error: 'At least one allowed domain is required while MCP is enabled' }
  }
  if (!value.excludedFolders || typeof value.excludedFolders !== 'object' || Array.isArray(value.excludedFolders)) {
    return { ok: false, error: 'excludedFolders must be an object' }
  }
  const excludedFolders = {}
  let total = 0
  for (const [rawDomain, rawFolders] of Object.entries(value.excludedFolders)) {
    const domain = Security.normalizeDomain(rawDomain)
    if (!domain || !allowedDomains.includes(domain)) {
      return { ok: false, forbidden: true, error: 'Excluded folders must belong to an allowed domain' }
    }
    if (!Array.isArray(rawFolders) || rawFolders.length > MAX_EXCLUDED_FOLDERS_PER_DOMAIN) {
      return { ok: false, error: `Each domain may exclude at most ${MAX_EXCLUDED_FOLDERS_PER_DOMAIN} folders` }
    }
    const folders = []
    for (const rawFolder of rawFolders) {
      const folder = normalizeCategoryPath(rawFolder)
      if (!folder) return { ok: false, error: 'Excluded folders must use safe bounded folder paths' }
      if (!folders.includes(folder)) folders.push(folder)
    }
    total += folders.length
    if (total > MAX_EXCLUDED_FOLDERS) {
      return { ok: false, error: `At most ${MAX_EXCLUDED_FOLDERS} folders may be excluded` }
    }
    if (folders.length) excludedFolders[domain] = folders
  }
  return {
    ok: true,
    preferences: {
      enabled: value.enabled,
      accessLevel: /** @type {'read-only'|'read-write'|'read-write-send'} */ (value.accessLevel),
      allowedDomains,
      excludedFolders,
    },
  }
}

/** @param {unknown} value @param {string[]} allowedDomains */
function normalizeExcludedFolders(value, allowedDomains) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const allowed = new Set(allowedDomains)
  const result = {}
  let total = 0
  for (const [rawDomain, rawFolders] of Object.entries(value)) {
    const domain = Security.normalizeDomain(rawDomain)
    if (!domain || !allowed.has(domain) || !Array.isArray(rawFolders)) continue
    const folders = []
    for (const rawFolder of rawFolders.slice(0, MAX_EXCLUDED_FOLDERS_PER_DOMAIN)) {
      const folder = normalizeCategoryPath(rawFolder)
      if (folder && !folders.includes(folder) && total < MAX_EXCLUDED_FOLDERS) {
        folders.push(folder)
        total++
      }
    }
    if (folders.length) result[domain] = folders
  }
  return result
}

/** @param {unknown} values @returns {string[]} */
function normalizeDomains(values) {
  if (!Array.isArray(values)) return []
  return [...new Set(values
    .map(value => Security.normalizeDomain(/** @type {string} */ (value)))
    .filter(/** @returns {value is string} */ value => value !== null))]
}
