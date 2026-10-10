import { createHash, randomBytes } from 'node:crypto'

export class Threading {
  static #MAX_MESSAGE_ID_LENGTH = 998

  static #MAX_REFERENCES = 100

  /**
   * Normalize one RFC 5322 Message-ID while rejecting control characters and
   * ambiguous multi-ID input. The canonical storage form always includes angle
   * brackets so comparisons do not depend on a sender's cosmetic formatting.
   *
   * @param {unknown} value
   * @returns {string | undefined}
   */
  static normalizeMessageId(value) {
    if (typeof value !== 'string') return undefined
    const input = value.trim()
    if (!input || input.length > Threading.#MAX_MESSAGE_ID_LENGTH + 2 || /[\r\n\u0000-\u001f\u007f]/.test(input)) return undefined
    const candidate = input.startsWith('<') && input.endsWith('>')
      ? input.slice(1, -1).trim()
      : input
    if (!candidate || candidate.length > Threading.#MAX_MESSAGE_ID_LENGTH || candidate.includes('<') || candidate.includes('>')) return undefined
    const at = candidate.lastIndexOf('@')
    if (at <= 0 || at === candidate.length - 1 || /\s/.test(candidate)) return undefined
    return `<${candidate}>`
  }

  /** Extract, normalize, de-duplicate, and cap a References-style ID list. */
  static parseMessageIdList(value) {
    const values = Array.isArray(value) ? value : typeof value === 'string' ? Threading.#extractCandidates(value) : []
    const normalized = []
    const seen = new Set()
    for (const candidate of values) {
      const id = Threading.normalizeMessageId(candidate)
      if (!id || seen.has(id)) continue
      seen.add(id)
      normalized.push(id)
      if (normalized.length >= Threading.#MAX_REFERENCES) break
    }
    return normalized
  }

  /** Build persisted thread metadata from normalized MIME headers. */
  static threadMetadataFromHeaders(headers, fallbackId, domain) {
    const rfcMessageId = Threading.parseMessageIdList(headers?.['message-id'])[0]
      ?? Threading.createMessageId(domain, fallbackId)
    const inReplyTo = Threading.parseMessageIdList(headers?.['in-reply-to'])[0]
    const references = Threading.parseMessageIdList(headers?.references)
    return Threading.#buildMetadata(domain, rfcMessageId, inReplyTo, references)
  }

  /** Build persisted thread metadata for a locally composed message. */
  static threadMetadataFromOutbound(message, fallbackId, domain) {
    const rfcMessageId = Threading.normalizeMessageId(message?.messageId)
      ?? Threading.createMessageId(domain, fallbackId)
    const inReplyTo = Threading.normalizeMessageId(message?.inReplyTo)
    const references = Threading.parseMessageIdList(message?.references)
    return Threading.#buildMetadata(domain, rfcMessageId, inReplyTo, references)
  }

  /** Generate an unguessable, standards-shaped Message-ID for local delivery. */
  static createMessageId(domain, hint = '') {
    const safeDomain = String(domain || 'localhost').toLowerCase().replace(/[^a-z0-9.-]/g, '') || 'localhost'
    const safeHint = String(hint || '').toLowerCase().replace(/[^a-z0-9._-]/g, '').slice(0, 80)
    const token = safeHint || randomBytes(16).toString('hex')
    return `<${token}.${randomBytes(8).toString('hex')}@${safeDomain}>`
  }

  /** Derive the same tenant-scoped identity used by persisted message metadata. */
  static deriveThreadId(domain, root) {
    return createHash('sha256').update(`${String(domain).toLowerCase()}\0${root}`).digest('hex')
  }

  /** @param {string} domain @param {string} rfcMessageId @param {string | undefined} inReplyTo @param {string[]} references */
  static #buildMetadata(domain, rfcMessageId, inReplyTo, references) {
    const lineage = [...references]
    if (inReplyTo && !lineage.includes(inReplyTo)) lineage.push(inReplyTo)
    const root = lineage[0] ?? rfcMessageId
    return {
      rfcMessageId,
      ...(inReplyTo ? { inReplyTo } : {}),
      ...(lineage.length ? { references: lineage } : {}),
      threadId: Threading.deriveThreadId(domain, root),
    }
  }

  /** @param {string} value */
  static #extractCandidates(value) {
    const bracketed = [...value.matchAll(/<([^<>\r\n]+)>/g)].map(match => `<${match[1]}>`)
    if (bracketed.length) return bracketed
    return value.split(/[\s,]+/).filter(Boolean)
  }
}
