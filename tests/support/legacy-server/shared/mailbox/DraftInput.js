import { Security } from '../security/Security.js'
import { MailValidation } from '../delivery/MailValidation.js'
import { Threading } from './Threading.js'

export class DraftInput {
  static #MAX_DRAFT_BODY_BYTES = 5 * 1024 * 1024

  /** Validate and normalize a complete autosave snapshot. */
  static parseDraftInput(body) {
    if (!body || typeof body !== 'object') return new Error('Draft payload required')
    const value = /** @type {Record<string, unknown>} */ (body)
    const clientId = typeof value.clientId === 'string' ? value.clientId.trim() : ''
    if (!clientId || clientId.length > 120 || !/^[A-Za-z0-9._:-]+$/.test(clientId)) {
      return new Error('clientId is required')
    }

    const to = DraftInput.#normalizeAddresses(value.to, 'to')
    if (to instanceof Error) return to
    const cc = DraftInput.#normalizeAddresses(value.cc, 'cc')
    if (cc instanceof Error) return cc
    const bcc = DraftInput.#normalizeAddresses(value.bcc, 'bcc')
    if (bcc instanceof Error) return bcc
    const replyTo = DraftInput.#normalizeAddresses(value.replyTo, 'replyTo', 1)
    if (replyTo instanceof Error) return replyTo
    let from
    if (value.from != null) {
      if (typeof value.from !== 'string') return new Error('from must be a valid owned email address')
      from = Security.normalizeEmailAddress(value.from)
      if (!from || Security.hasControlChars(String(value.from))) return new Error('from must be a valid owned email address')
    }

    const subject = typeof value.subject === 'string' ? value.subject.trim() : ''
    if (subject.length > 500 || Security.hasControlChars(subject)) return new Error('subject is invalid')
    const text = typeof value.text === 'string' ? value.text : ''
    const html = typeof value.html === 'string' ? value.html : ''
    if (new TextEncoder().encode(text).byteLength + new TextEncoder().encode(html).byteLength > DraftInput.#MAX_DRAFT_BODY_BYTES) {
      return new Error('Draft body is too large (max 5MB)')
    }

    let inReplyTo
    if (value.inReplyTo != null) {
      inReplyTo = Threading.normalizeMessageId(value.inReplyTo)
      if (!inReplyTo) return new Error('inReplyTo must be a valid Message-ID')
    }
    let references = []
    if (value.references != null) {
      if (!Array.isArray(value.references) || value.references.length > 100 || value.references.some(item => typeof item !== 'string')) {
        return new Error('references must be an array of at most 100 Message-IDs')
      }
      references = Threading.parseMessageIdList(value.references)
      if (references.length !== value.references.length) return new Error('references must contain valid unique Message-IDs')
    }

    let sourceEmailId
    if (value.sourceEmailId != null) {
      if (typeof value.sourceEmailId !== 'string' || !value.sourceEmailId.trim() || value.sourceEmailId.length > 200 || /[\r\n]/.test(value.sourceEmailId)) {
        return new Error('sourceEmailId is invalid')
      }
      sourceEmailId = value.sourceEmailId.trim()
    }

    const parsedAttachments = MailValidation.validateSendAttachments(value.attachments ?? [])
    if (!parsedAttachments.ok) return new Error(parsedAttachments.error)
    return {
      clientId, ...(from ? { from } : {}), to, cc, bcc, replyTo, subject, text, html,
      ...(inReplyTo ? { inReplyTo } : {}),
      ...(sourceEmailId ? { sourceEmailId } : {}),
      references,
      attachments: parsedAttachments.value,
    }
  }

  /** @param {unknown} value @param {string} name @param {number} [maximum] */
  static #normalizeAddresses(value, name, maximum = 100) {
    if (value == null) return []
    if (!Array.isArray(value) || value.length > maximum) return new Error(`${name} must be an array of at most ${maximum} addresses`)
    const normalized = []
    for (const item of value) {
      const address = Security.normalizeEmailAddress(item)
      if (!address || Security.hasControlChars(String(item))) return new Error(`${name} contains an invalid email address`)
      normalized.push(address)
    }
    return [...new Set(normalized)]
  }
}
