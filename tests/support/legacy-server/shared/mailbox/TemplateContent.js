export class TemplateContent {
  static #NAME_RE = /^[\p{L}\p{N}_\- ]+$/u

  static #MAX_NAME = 100

  static #MAX_SUBJECT = 998

  static #MAX_ADDRESSES = 8_192

  static #MAX_HTML = 512 * 1024

  static #UNSAFE_BLOCK_RE = /<(script|style|iframe|frame|frameset|object|embed|applet|form|template|noscript)\b[^>]*>[\s\S]*?<\/\1\s*>/gi

  static #UNSAFE_SINGLE_RE = /<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*\/?\s*>/gi

  /** @param {unknown} input */
  static sanitizeStoredTemplateHTML(input) {
    return String(input ?? '')
      .replace(/\u0000/g, '')
      .replace(TemplateContent.#UNSAFE_BLOCK_RE, '')
      .replace(TemplateContent.#UNSAFE_SINGLE_RE, '')
      .replace(/\s+on[a-z0-9_-]+\s*=\s*(?:\x22[^\x22]*\x22|\x27[^\x27]*\x27|[^\s>]+)/gi, '')
      .replace(/\s+(href|src|xlink:href)\s*=\s*([\x22\x27])\s*(?:javascript|vbscript):[\s\S]*?\2/gi, ' $1="#"')
  }

  /**
   * Validate the owner-editable template fields and ignore identity fields.
   *
   * @param {unknown} input
   * @param {{ partial?: boolean }} [options]
   * @returns {Record<string, string> | Error}
   */
  static normalizeTemplateInput(input, { partial = false } = {}) {
    if (!input || typeof input !== 'object' || Array.isArray(input)) return new Error('Template payload is required')
    const source = /** @type {Record<string, unknown>} */ (input)
    const patch = TemplateContent.#copyTemplateFields(source)
    if (patch instanceof Error) return patch
    for (const validate of [TemplateContent.#normalizeName, TemplateContent.#normalizeSubject, TemplateContent.#normalizeText, TemplateContent.#normalizeAddresses]) {
      const error = validate(patch, partial)
      if (error) return error
    }
    if (partial && Object.keys(patch).length === 0) return new Error('At least one template field is required')
    return patch
  }

  /** @param {Record<string, unknown>} source @returns {Record<string, string> | Error} */
  static #copyTemplateFields(source) {
    const patch = {}
    for (const key of ['name', 'subject', 'text', 'to', 'cc']) {
      if (!(key in source)) continue
      if (typeof source[key] !== 'string') return new Error(`${key} must be text`)
      patch[key] = source[key]
    }
    return patch
  }

  /** @param {Record<string, string>} patch @param {boolean} partial */
  static #normalizeName(patch, partial) {
    if (partial && !('name' in patch)) return null
    const name = String(patch.name ?? '').trim().replace(/\s+/g, ' ')
    if (!name) return new Error('name is required')
    if (name.length > TemplateContent.#MAX_NAME || !TemplateContent.#NAME_RE.test(name)) {
      return new Error('name may contain letters, numbers, spaces, dashes, and underscores (100 characters maximum)')
    }
    patch.name = name
    return null
  }

  /** @param {Record<string, string>} patch @param {boolean} partial */
  static #normalizeSubject(patch, partial) {
    if (partial && !('subject' in patch)) return null
    const subject = String(patch.subject ?? '').trim()
    if (subject.length > TemplateContent.#MAX_SUBJECT || /[\r\n\u0000]/.test(subject)) return new Error('subject is invalid')
    patch.subject = subject
    return null
  }

  /** @param {Record<string, string>} patch @param {boolean} partial */
  static #normalizeText(patch, partial) {
    if (partial && !('text' in patch)) return null
    const text = TemplateContent.sanitizeStoredTemplateHTML(patch.text ?? '')
    if (!partial && !text.trim()) return new Error('template content is required')
    if (Buffer.byteLength(text, 'utf8') > TemplateContent.#MAX_HTML) return new Error('template content exceeds 512 KiB')
    patch.text = text
    return null
  }

  /** @param {Record<string, string>} patch @param {boolean} partial */
  static #normalizeAddresses(patch, partial) {
    for (const key of ['to', 'cc']) {
      if (partial && !(key in patch)) continue
      const value = String(patch[key] ?? '').trim()
      if (value.length > TemplateContent.#MAX_ADDRESSES || /[\r\n\u0000]/.test(value)) return new Error(`${key} is invalid`)
      patch[key] = value
    }
    return null
  }

  /** @param {string} requested @param {Array<{id?: string, name?: string}>} records @param {string} [excludeId] */
  static resolveUniqueTemplateName(requested, records, excludeId = '') {
    const used = new Set(records.filter(item => item.id !== excludeId).map(item => String(item.name || '').toLocaleLowerCase()))
    if (!used.has(requested.toLocaleLowerCase())) return requested
    let suffix = 1
    while (used.has(`${requested} ${suffix}`.toLocaleLowerCase())) suffix += 1
    return `${requested} ${suffix}`
  }
}
