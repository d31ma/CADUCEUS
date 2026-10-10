import { Security } from '../security/Security.js'

const NAME_RE = /^[\p{L}\p{N}_\- ]+$/u
const MAX_NAME = 100
const MAX_HTML = 512 * 1024
const UNSAFE_BLOCK_RE = /<(script|style|iframe|frame|frameset|object|embed|applet|form|template|noscript)\b[^>]*>[\s\S]*?<\/\1\s*>/gi
const UNSAFE_SINGLE_RE = /<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*\/?\s*>/gi

/** @param {unknown} value @returns {string | Error} */
function normalizeName(value) {
  if (typeof value !== 'string') return new Error('name must be text')
  const name = value.trim().replace(/\s+/g, ' ')
  if (!name) return new Error('name is required')
  if (name.length > MAX_NAME || !NAME_RE.test(name)) {
    return new Error('name may contain letters, numbers, spaces, dashes, and underscores (100 characters maximum)')
  }
  return name
}

/** @param {unknown} value @returns {string | Error} */
function normalizeText(value) {
  if (typeof value !== 'string') return new Error('text must be HTML or plain text')
  const text = sanitizeSignatureHTML(value)
  if (!text.trim()) return new Error('signature content is required')
  if (Buffer.byteLength(text, 'utf8') > MAX_HTML) return new Error('signature content exceeds 512 KiB')
  return text
}

/** @param {unknown} value @param {string[]} allowedAddresses @returns {string[] | Error} */
function normalizeDefaults(value, allowedAddresses) {
  const requested = value ?? []
  if (!Array.isArray(requested) || requested.length > 100) {
    return new Error('defaultFor must be an array of owned email addresses')
  }
  const defaults = []
  for (const address of requested) {
    const normalized = Security.normalizeEmailAddress(address)
    if (!normalized || !allowedAddresses.includes(normalized)) {
      return new Error('defaultFor contains an address not owned by this mailbox')
    }
    if (!defaults.includes(normalized)) defaults.push(normalized)
  }
  return defaults
}

/** @param {Record<string, unknown>} source @param {string[]} allowedAddresses */
function requestedDefaults(source, allowedAddresses) {
  if (source.defaultFor != null || typeof source.domain !== 'string') return source.defaultFor
  const domain = Security.normalizeDomain(source.domain)
  return allowedAddresses.filter(address => address.endsWith(`@${domain}`))
}

/** @param {unknown} input */
export function sanitizeSignatureHTML(input) {
  return String(input ?? '')
    .replace(/\u0000/g, '')
    .replace(UNSAFE_BLOCK_RE, '')
    .replace(UNSAFE_SINGLE_RE, '')
    .replace(/\s+on[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+(href|src|xlink:href)\s*=\s*(["'])\s*(?:javascript|vbscript):[\s\S]*?\2/gi, ' $1="#"')
    .replace(/\s+style\s*=\s*(["'])[^"']*(?:expression\s*\(|url\s*\(\s*["']?\s*(?:javascript|vbscript):)[^"']*\1/gi, '')
}

/**
 * Validate owner-editable fields while ignoring identity and timestamps.
 * `domain` remains an accepted compatibility hint; defaults are always
 * materialized as exact owned addresses.
 *
 * @param {unknown} input
 * @param {{ partial?: boolean, allowedAddresses: string[] }} options
 * @returns {{ name?: string, text?: string, defaultFor?: string[] } | Error}
 */
export function normalizeSignatureInput(input, { partial = false, allowedAddresses }) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return new Error('Signature payload is required')
  const source = /** @type {Record<string, unknown>} */ (input)
  const result = {}

  if (!partial || 'name' in source) {
    const name = normalizeName(source.name)
    if (name instanceof Error) return name
    result.name = name
  }

  if (!partial || 'text' in source) {
    const text = normalizeText(source.text)
    if (text instanceof Error) return text
    result.text = text
  }

  if (!partial || 'defaultFor' in source || 'domain' in source) {
    const defaults = normalizeDefaults(requestedDefaults(source, allowedAddresses), allowedAddresses)
    if (defaults instanceof Error) return defaults
    result.defaultFor = defaults
  }

  if (partial && Object.keys(result).length === 0) return new Error('At least one signature field is required')
  return result
}

/** @param {string} requested @param {Array<{id?: string, name?: string}>} records @param {string} [excludeId] */
export function resolveUniqueSignatureName(requested, records, excludeId = '') {
  const used = new Set(records.filter(item => item.id !== excludeId).map(item => String(item.name || '').toLocaleLowerCase()))
  if (!used.has(requested.toLocaleLowerCase())) return requested
  let suffix = 1
  while (used.has(`${requested} ${suffix}`.toLocaleLowerCase())) suffix += 1
  return `${requested} ${suffix}`
}
