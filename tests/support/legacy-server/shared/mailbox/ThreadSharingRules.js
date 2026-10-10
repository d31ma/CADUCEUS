import { createHash, randomBytes, timingSafeEqual } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { EmailsRecords } from './EmailsRecords.js'
import { FyloAccessContext } from '../persistence/FyloAccessContext.js'
import { Security } from '../security/Security.js'

export class ThreadSharingRules {
  static EXPIRY_DAYS = new Set([1, 7, 30])

  static TOKEN_SECRET_BYTES = 32

  static MAX_EMAIL_IDS_JSON_BYTES = 512 * 1024

  /** @param {string} domain */
  static createToken(domain) {
    return `${Buffer.from(domain, 'utf8').toString('base64url')}.${randomBytes(ThreadSharingRules.TOKEN_SECRET_BYTES).toString('base64url')}`
  }

  /** @param {unknown} token */
  static parseToken(token) {
    const value = typeof token === 'string' ? token : ''
    const match = value.match(/^([A-Za-z0-9_-]{4,340})\.([A-Za-z0-9_-]{43})$/)
    if (!match || value.length > 400) return null
    let decoded
    try { decoded = Buffer.from(match[1], 'base64url').toString('utf8') } catch { return null }
    const domain = Security.normalizeDomain(decoded)
    if (!domain || Buffer.from(domain, 'utf8').toString('base64url') !== match[1]) return null
    return { domain, hash: ThreadSharingRules.hashToken(value) }
  }

  /** @param {string} token */
  static hashToken(token) { return createHash('sha256').update(token).digest('hex') }

  /** @param {string} left @param {string} right */
  static equalHash(left, right) {
    if (!/^[a-f0-9]{64}$/.test(left) || !/^[a-f0-9]{64}$/.test(right)) return false
    return timingSafeEqual(Buffer.from(left, 'hex'), Buffer.from(right, 'hex'))
  }

  /** @param {{ expiresAt: string, revokedAt?: string }} record @param {Date} now */
  static isActive(record, now) {
    const expiresAt = Date.parse(record.expiresAt)
    return !record.revokedAt && Number.isFinite(expiresAt) && expiresAt > now.getTime()
  }

  /** @param {any} record */
  static presentManaged(record) {
    return {
      id: record.id,
      threadId: record.threadId,
      subject: record.subject,
      messageCount: record.emailIds.length,
      createdAt: record.createdAt,
      expiresAt: record.expiresAt,
      revokedAt: record.revokedAt || '',
      active: ThreadSharingRules.isActive(record, new Date()),
    }
  }

  /** @param {any} email @param {any[]} attachments */
  static presentPublicMessage(email, attachments) {
    return {
      from: String(email.headerFrom || email.sender || ''),
      to: Array.isArray(email.to) && email.to.length ? email.to : [String(email.recipient || '')].filter(Boolean),
      cc: Array.isArray(email.cc) ? email.cc : [],
      subject: String(email.subject || ''),
      body: String(email.body || ''),
      htmlBody: String(email.htmlBody || ''),
      receivedAt: String(email.receivedAt || ''),
      attachments: attachments.map(attachment => ({
        id: attachment.id,
        filename: attachment.filename,
        contentType: attachment.contentType,
        size: attachment.size,
      })),
    }
  }
}
