import { Buffer } from 'node:buffer'
import { Security } from '../../../platform/security/security.js'
import { normalizeMessageId, parseMessageIdList } from '../../mailbox/services/threading.js'
import { normalizeAttachmentContentId, sanitizeAttachmentFilename } from '../../mailbox/services/attachment-policy.js'

const MAX_ATTACHMENTS = 10
const MAX_ATTACHMENT_BYTES = 10 * 1024 * 1024
const MAX_TOTAL_ATTACHMENT_BYTES = 25 * 1024 * 1024
const MAX_RECIPIENTS = 100
const MAX_TO_FIELD = 50

/** Validation for outbound send requests (recipients, subject, attachments). */
export class MailValidation {
  /**
   * @param {unknown} to
   * @returns {{ valid: string[], errors: string[] }}
   */
  static validateRecipients(to) {
    if (!Array.isArray(to) || to.length === 0) return { valid: [], errors: ['Missing to field'] }
    if (to.length > MAX_RECIPIENTS) return { valid: [], errors: [`Too many recipients (max ${MAX_RECIPIENTS})`] }
    const valid = []
    const errors = []
    for (const addr of to) {
      if (typeof addr !== 'string' || !addr.trim()) { errors.push('Invalid recipient'); continue }
      const normalized = Security.normalizeEmailAddress(addr)
      if (!normalized || Security.hasControlChars(normalized)) { errors.push(`Invalid email: ${addr}`); continue }
      valid.push(normalized)
    }
    return { valid, errors }
  }

  /** @param {string} subject @returns {string | null} */
  static validateSubject(subject) {
    if (typeof subject !== 'string' || !subject.trim()) return 'Subject is required'
    if (Security.hasControlChars(subject)) return 'Subject contains invalid characters'
    if (/[\r\n]/.test(subject)) return 'Subject must not contain newlines'
    return null
  }

  /** @param {string} subject @returns {boolean} */
  static hasHeaderInjection(subject) {
    return /^(?:bcc|cc|to|from|reply-to|sender|return-path|message-id|mime-version|content-type|content-transfer-encoding):/im.test(subject.trim())
  }

  /**
   * @param {unknown} attachments
   * @returns {{ ok: true, value: import('../../../types').SendAttachment[] } | { ok: false, error: string }}
   */
  static validateSendAttachments(attachments) {
    if (attachments == null) return { ok: true, value: [] }
    if (!Array.isArray(attachments)) return { ok: false, error: 'attachments must be an array' }
    if (attachments.length > MAX_ATTACHMENTS) return { ok: false, error: `Too many attachments (max ${MAX_ATTACHMENTS})` }

    let totalBytes = 0
    const value = []
    const contentIds = new Set()
    for (const item of attachments) {
      const parsed = MailValidation.#validateAttachmentItem(item, totalBytes)
      if (!parsed.ok) return parsed
      const contentId = parsed.value.contentId?.toLowerCase()
      if (contentId && contentIds.has(contentId)) return { ok: false, error: 'Attachment contentId values must be unique' }
      if (contentId) contentIds.add(contentId)
      totalBytes += parsed.size
      value.push(parsed.value)
    }
    return { ok: true, value }
  }

  /**
   * @param {unknown} body
   * @returns {{ ok: true, value: { from?: string, to: string[], cc?: string[], bcc?: string[], subject: string, text?: string, html?: string, replyTo?: string[], inReplyTo?: string, references?: string[], openTracking?: boolean, linkTracking?: boolean } } | { ok: false, error: string }}
   */
  static validateSendRequest(body) {
    if (!body || typeof body !== 'object') return { ok: false, error: 'Request body required' }
    const b = /** @type {{ from?: unknown, subject?: string, to?: string[], cc?: string[], bcc?: string[], replyTo?: unknown, inReplyTo?: unknown, references?: unknown, text?: string, html?: string, openTracking?: unknown, linkTracking?: unknown }} */ (body)
    if (!b.subject || typeof b.subject !== 'string' || !b.subject.trim()) return { ok: false, error: 'Subject is required' }
    if (MailValidation.hasHeaderInjection(b.subject)) return { ok: false, error: 'Subject contains invalid characters' }
    const { valid, errors } = MailValidation.validateRecipients(b.to)
    if (errors.length) return { ok: false, error: errors[0] }
    if (valid.length > MAX_TO_FIELD) return { ok: false, error: `Too many recipients (max ${MAX_TO_FIELD})` }
    if (!b.text && !b.html) return { ok: false, error: 'text or html required' }
    const cc = MailValidation.#validateOptionalRecipients(b.cc, 'cc')
    if (!cc.ok) return cc
    const bcc = MailValidation.#validateOptionalRecipients(b.bcc, 'bcc')
    if (!bcc.ok) return bcc
    if (valid.length + cc.value.length + bcc.value.length > MAX_RECIPIENTS) {
      return { ok: false, error: `Too many recipients (max ${MAX_RECIPIENTS})` }
    }
    if (b.openTracking != null && typeof b.openTracking !== 'boolean') {
      return { ok: false, error: 'openTracking must be a boolean' }
    }
    if (b.linkTracking != null && typeof b.linkTracking !== 'boolean') {
      return { ok: false, error: 'linkTracking must be a boolean' }
    }
    if ((b.openTracking === true || b.linkTracking === true) && !b.html) {
      return { ok: false, error: 'Tracking requires an HTML body' }
    }
    let from
    if (b.from != null) {
      if (typeof b.from !== 'string') return { ok: false, error: 'from must be a valid owned email address' }
      from = Security.normalizeEmailAddress(b.from)
      if (!from || Security.hasControlChars(String(b.from))) return { ok: false, error: 'from must be a valid owned email address' }
    }
    let replyTo
    if (b.replyTo != null) {
      if (!Array.isArray(b.replyTo) || b.replyTo.length !== 1) {
        return { ok: false, error: 'replyTo must contain exactly one email address' }
      }
      const normalized = Security.normalizeEmailAddress(b.replyTo[0])
      if (!normalized || Security.hasControlChars(String(b.replyTo[0]))) {
        return { ok: false, error: 'replyTo must contain a valid email address' }
      }
      replyTo = [normalized]
    }
    let inReplyTo
    if (b.inReplyTo != null) {
      inReplyTo = normalizeMessageId(b.inReplyTo)
      if (!inReplyTo) return { ok: false, error: 'inReplyTo must be a valid Message-ID' }
    }
    let references
    if (b.references != null) {
      if (!Array.isArray(b.references) || b.references.length > 100 || b.references.some(value => typeof value !== 'string')) {
        return { ok: false, error: 'references must be an array of at most 100 Message-IDs' }
      }
      references = parseMessageIdList(b.references)
      if (references.length !== b.references.length) {
        return { ok: false, error: 'references must contain valid unique Message-IDs' }
      }
    }
    return {
      ok: true,
      value: {
        ...(from ? { from } : {}),
        to: valid,
        ...(cc.value.length ? { cc: cc.value } : {}),
        ...(bcc.value.length ? { bcc: bcc.value } : {}),
        ...(replyTo ? { replyTo } : {}),
        ...(inReplyTo ? { inReplyTo } : {}),
        ...(references?.length ? { references } : {}),
        subject: b.subject.trim(),
        ...(b.text ? { text: b.text } : {}),
        ...(b.html ? { html: b.html } : {}),
        ...(b.openTracking === true ? { openTracking: true } : {}),
        ...(b.linkTracking === true ? { linkTracking: true } : {}),
      },
    }
  }

  /**
   * @param {unknown} value
   * @param {'cc'|'bcc'} field
   * @returns {{ ok: true, value: string[] } | { ok: false, error: string }}
   */
  static #validateOptionalRecipients(value, field) {
    if (value == null) return { ok: true, value: [] }
    if (!Array.isArray(value)) return { ok: false, error: `${field} must be an array` }
    if (value.length > MAX_RECIPIENTS) return { ok: false, error: `Too many recipients (max ${MAX_RECIPIENTS})` }
    const normalized = []
    for (const address of value) {
      if (typeof address !== 'string') return { ok: false, error: `${field} must contain valid email addresses` }
      const email = Security.normalizeEmailAddress(address)
      if (!email || Security.hasControlChars(address)) return { ok: false, error: `${field} must contain valid email addresses` }
      normalized.push(email)
    }
    return { ok: true, value: normalized }
  }

  /**
   * @param {unknown} item
   * @param {number} totalSoFar
   * @returns {{ ok: true, value: import('../../../types').SendAttachment, size: number } | { ok: false, error: string }}
   */
  static #validateAttachmentItem(item, totalSoFar) {
    if (!item || typeof item !== 'object') return { ok: false, error: 'Invalid attachment' }
    const a = /** @type {{ filename?: unknown, contentType?: unknown, contentBase64?: unknown, disposition?: unknown, contentId?: unknown }} */ (item)

    if (typeof a.filename !== 'string' || !a.filename.trim()) return { ok: false, error: 'Attachment filename is required' }
    if (Security.hasControlChars(a.filename)) return { ok: false, error: 'Attachment filename contains invalid characters' }
    if (typeof a.contentBase64 !== 'string' || !a.contentBase64.trim()) return { ok: false, error: 'Attachment contentBase64 is required' }
    if (!/^[A-Za-z0-9+/]+={0,2}$/.test(a.contentBase64)) return { ok: false, error: 'Attachment contentBase64 must be valid base64' }

    const contentType = typeof a.contentType === 'string' && a.contentType.trim()
      ? a.contentType.trim().toLowerCase()
      : 'application/octet-stream'
    if (!/^[a-z]+\/[a-z0-9.+-]+$/.test(contentType)) return { ok: false, error: 'Attachment contentType is invalid' }
    const contentId = normalizeAttachmentContentId(a.contentId)
    if (a.contentId != null && !contentId) return { ok: false, error: 'Attachment contentId is invalid' }

    const size = Buffer.from(a.contentBase64, 'base64').byteLength
    if (size <= 0) return { ok: false, error: 'Attachment content is empty' }
    if (size > MAX_ATTACHMENT_BYTES) return { ok: false, error: 'Attachment is too large (max 10MB)' }
    if (totalSoFar + size > MAX_TOTAL_ATTACHMENT_BYTES) return { ok: false, error: 'Attachments are too large (max 25MB total)' }

    return {
      ok: true,
      size,
      value: {
        filename: sanitizeAttachmentFilename(a.filename),
        contentType,
        contentBase64: a.contentBase64,
        disposition: a.disposition === 'inline' ? 'inline' : 'attachment',
        ...(contentId ? { contentId } : {}),
      },
    }
  }
}
