const NAME_RE = /^[\p{L}\p{N}_\- ]+$/u
const MAX_NAME = 100
const MAX_SUBJECT = 998
const MAX_ADDRESSES = 8_192
const MAX_HTML = 512 * 1024
const UNSAFE_BLOCK_RE = /<(script|style|iframe|frame|frameset|object|embed|applet|form|template|noscript)\b[^>]*>[\s\S]*?<\/\1\s*>/gi
const UNSAFE_SINGLE_RE = /<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*\/?\s*>/gi

/** @param {unknown} input */
export function sanitizeStoredTemplateHTML(input) {
  return String(input ?? '')
    .replace(/\u0000/g, '')
    .replace(UNSAFE_BLOCK_RE, '')
    .replace(UNSAFE_SINGLE_RE, '')
    .replace(/\s+on[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+(href|src|xlink:href)\s*=\s*(["'])\s*(?:javascript|vbscript):[\s\S]*?\2/gi, ' $1="#"')
}

/**
 * Validate the owner-editable template fields and ignore identity fields.
 *
 * @param {unknown} input
 * @param {{ partial?: boolean }} [options]
 * @returns {Record<string, string> | Error}
 */
export function normalizeTemplateInput(input, { partial = false } = {}) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return new Error('Template payload is required')
  const source = /** @type {Record<string, unknown>} */ (input)
  const patch = copyTemplateFields(source)
  if (patch instanceof Error) return patch
  for (const validate of [normalizeName, normalizeSubject, normalizeText, normalizeAddresses]) {
    const error = validate(patch, partial)
    if (error) return error
  }
  if (partial && Object.keys(patch).length === 0) return new Error('At least one template field is required')
  return patch
}

/** @param {Record<string, unknown>} source @returns {Record<string, string> | Error} */
function copyTemplateFields(source) {
  const patch = {}
  for (const key of ['name', 'subject', 'text', 'to', 'cc']) {
    if (!(key in source)) continue
    if (typeof source[key] !== 'string') return new Error(`${key} must be text`)
    patch[key] = source[key]
  }
  return patch
}

/** @param {Record<string, string>} patch @param {boolean} partial */
function normalizeName(patch, partial) {
  if (partial && !('name' in patch)) return null
  const name = String(patch.name ?? '').trim().replace(/\s+/g, ' ')
  if (!name) return new Error('name is required')
  if (name.length > MAX_NAME || !NAME_RE.test(name)) {
    return new Error('name may contain letters, numbers, spaces, dashes, and underscores (100 characters maximum)')
  }
  patch.name = name
  return null
}

/** @param {Record<string, string>} patch @param {boolean} partial */
function normalizeSubject(patch, partial) {
  if (partial && !('subject' in patch)) return null
  const subject = String(patch.subject ?? '').trim()
  if (subject.length > MAX_SUBJECT || /[\r\n\u0000]/.test(subject)) return new Error('subject is invalid')
  patch.subject = subject
  return null
}

/** @param {Record<string, string>} patch @param {boolean} partial */
function normalizeText(patch, partial) {
  if (partial && !('text' in patch)) return null
  const text = sanitizeStoredTemplateHTML(patch.text ?? '')
  if (!partial && !text.trim()) return new Error('template content is required')
  if (Buffer.byteLength(text, 'utf8') > MAX_HTML) return new Error('template content exceeds 512 KiB')
  patch.text = text
  return null
}

/** @param {Record<string, string>} patch @param {boolean} partial */
function normalizeAddresses(patch, partial) {
  for (const key of ['to', 'cc']) {
    if (partial && !(key in patch)) continue
    const value = String(patch[key] ?? '').trim()
    if (value.length > MAX_ADDRESSES || /[\r\n\u0000]/.test(value)) return new Error(`${key} is invalid`)
    patch[key] = value
  }
  return null
}

/** @param {string} requested @param {Array<{id?: string, name?: string}>} records @param {string} [excludeId] */
export function resolveUniqueTemplateName(requested, records, excludeId = '') {
  const used = new Set(records.filter(item => item.id !== excludeId).map(item => String(item.name || '').toLocaleLowerCase()))
  if (!used.has(requested.toLocaleLowerCase())) return requested
  let suffix = 1
  while (used.has(`${requested} ${suffix}`.toLocaleLowerCase())) suffix += 1
  return `${requested} ${suffix}`
}
