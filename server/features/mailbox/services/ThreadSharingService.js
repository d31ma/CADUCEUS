import { createHash, randomBytes, timingSafeEqual } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { canAccessEmail } from '../repositories/EmailsRepository.js'
import { FyloAccessContext } from '../../../shared/persistence/FyloAccessContext.js'
import { Security } from '../../../shared/security/Security.js'

const EXPIRY_DAYS = new Set([1, 7, 30])
const TOKEN_SECRET_BYTES = 32
const MAX_EMAIL_IDS_JSON_BYTES = 512 * 1024

/**
 * Creates and resolves self-hosted conversation-sharing capabilities.
 * The bearer secret is returned once and never persisted; FYLO stores only a
 * SHA-256 hash, exact mailbox ownership, immutable message membership, expiry,
 * and revocation state.
 */
export class ThreadSharingService {
  /**
   * @param {import('../repositories/ThreadSharesRepository.js').ThreadSharesRepository} shares
   * @param {import('../repositories/ThreadsRepository.js').ThreadsRepository} threads
   * @param {import('../repositories/EmailsRepository.js').EmailsRepository} emails
   * @param {import('../repositories/AttachmentsRepository.js').AttachmentsRepository} attachments
   */
  constructor(shares, threads, emails, attachments) {
    this.shares = shares
    this.threads = threads
    this.emails = emails
    this.attachments = attachments
  }

  /**
   * @param {{ ownerEmail: string, domains: string[], emailId: string, expiresInDays: number }} input
   */
  async create(input) {
    const expiresInDays = Number(input.expiresInDays)
    if (!Number.isInteger(expiresInDays) || !EXPIRY_DAYS.has(expiresInDays)) {
      throw new ThreadShareInputError('expiresInDays must be one of: 1, 7, 30')
    }
    const emailId = String(input.emailId || '').trim()
    if (!emailId || emailId.length > 500) throw new ThreadShareInputError('emailId is required')

    const thread = await this.threads.findForEmail(input.domains, emailId, input.ownerEmail)
    if (!thread) return null
    const domain = Security.normalizeDomain(thread.latestEmail.domain)
    if (!domain || !input.domains.includes(domain)) return null

    const emailIds = thread.emails.map(email => email.id)
    const serializedIds = JSON.stringify(emailIds)
    if (Buffer.byteLength(serializedIds) > MAX_EMAIL_IDS_JSON_BYTES) {
      throw new ThreadShareInputError('This conversation is too large to share safely')
    }

    const now = new Date()
    const revokedAt = now.toISOString()
    const existing = await this.shares.listForOwner(input.ownerEmail)
    await Promise.all(existing
      .filter(share => share.threadId === thread.id && isActive(share, now))
      .map(share => this.shares.update(share.docId, { revokedAt })))

    const token = createToken(domain)
    const record = {
      id: `share-${randomBytes(16).toString('base64url')}`,
      ownerEmail: input.ownerEmail.toLowerCase(),
      domain,
      threadId: thread.id,
      subject: String(thread.subject || '(No subject)').slice(0, 500),
      emailIds,
      tokenHash: hashToken(token),
      createdAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + expiresInDays * 86_400_000).toISOString(),
    }
    await this.shares.put(record)
    return { ...presentManaged(record), token, url: `/share/${encodeURIComponent(token)}` }
  }

  /** @param {string} ownerEmail @param {string} [emailId] @param {string[]} [domains] */
  async list(ownerEmail, emailId, domains = []) {
    const records = await this.shares.listForOwner(ownerEmail)
    let selected = records
    if (emailId) {
      const thread = await this.threads.findForEmail(domains, emailId, ownerEmail)
      if (!thread) return null
      selected = records.filter(share => share.threadId === thread.id)
    }
    return selected
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
      .map(presentManaged)
  }

  /** @param {string} id @param {string} ownerEmail */
  async revoke(id, ownerEmail) {
    const [docId, record] = await this.shares.findById(id)
    if (!docId || !record || record.ownerEmail !== ownerEmail.toLowerCase()) return false
    if (!record.revokedAt) await this.shares.update(docId, { revokedAt: new Date().toISOString() })
    return true
  }

  /** Resolve and re-authorize every message each time a public capability is used. */
  async resolve(token) {
    const parsed = parseToken(token)
    if (!parsed) return null
    return await FyloAccessContext.runForDomain(parsed.domain, async () => {
      const records = await this.shares.findByTokenHash(parsed.hash)
      const record = records.find(candidate =>
        candidate.domain === parsed.domain && equalHash(candidate.tokenHash, parsed.hash))
      if (!record || !isActive(record, new Date())) return null

      const messages = []
      for (const emailId of record.emailIds) {
        const [, email] = await this.emails.findById(emailId)
        if (!email || email.domain !== record.domain || !canAccessEmail(email, record.ownerEmail)) continue
        const attachments = await this.attachments.listSummaries(email.id)
        messages.push(presentPublicMessage(email, attachments))
      }
      messages.sort((a, b) => a.receivedAt.localeCompare(b.receivedAt))
      if (messages.length === 0) return null
      return {
        id: record.id,
        subject: record.subject,
        createdAt: record.createdAt,
        expiresAt: record.expiresAt,
        messages,
      }
    })
  }

  /** Attachment reads repeat full capability and immutable membership checks. */
  async readAttachment(token, attachmentId) {
    if (typeof attachmentId !== 'string' || !attachmentId || attachmentId.length > 200) return null
    const parsed = parseToken(token)
    if (!parsed) return null
    return await FyloAccessContext.runForDomain(parsed.domain, async () => {
      const records = await this.shares.findByTokenHash(parsed.hash)
      const record = records.find(candidate =>
        candidate.domain === parsed.domain && equalHash(candidate.tokenHash, parsed.hash))
      if (!record || !isActive(record, new Date())) return null

      const [, attachment] = await this.attachments.findById(attachmentId)
      if (!attachment || attachment.domain !== record.domain || !record.emailIds.includes(attachment.emailId)) return null
      const [, email] = await this.emails.findById(attachment.emailId)
      if (!email || !canAccessEmail(email, record.ownerEmail)) return null
      const bytes = await this.attachments.readContent(attachment)
      return {
        id: attachment.id,
        filename: attachment.filename,
        contentType: attachment.contentType,
        size: attachment.size,
        contentBase64: Buffer.from(bytes).toString('base64'),
      }
    })
  }
}

export class ThreadShareInputError extends Error {}

/** @param {string} domain */
function createToken(domain) {
  return `${Buffer.from(domain, 'utf8').toString('base64url')}.${randomBytes(TOKEN_SECRET_BYTES).toString('base64url')}`
}

/** @param {unknown} token */
function parseToken(token) {
  const value = typeof token === 'string' ? token : ''
  const match = value.match(/^([A-Za-z0-9_-]{4,340})\.([A-Za-z0-9_-]{43})$/)
  if (!match || value.length > 400) return null
  let decoded
  try { decoded = Buffer.from(match[1], 'base64url').toString('utf8') } catch { return null }
  const domain = Security.normalizeDomain(decoded)
  if (!domain || Buffer.from(domain, 'utf8').toString('base64url') !== match[1]) return null
  return { domain, hash: hashToken(value) }
}

/** @param {string} token */
function hashToken(token) { return createHash('sha256').update(token).digest('hex') }

/** @param {string} left @param {string} right */
function equalHash(left, right) {
  if (!/^[a-f0-9]{64}$/.test(left) || !/^[a-f0-9]{64}$/.test(right)) return false
  return timingSafeEqual(Buffer.from(left, 'hex'), Buffer.from(right, 'hex'))
}

/** @param {{ expiresAt: string, revokedAt?: string }} record @param {Date} now */
function isActive(record, now) {
  const expiresAt = Date.parse(record.expiresAt)
  return !record.revokedAt && Number.isFinite(expiresAt) && expiresAt > now.getTime()
}

/** @param {any} record */
function presentManaged(record) {
  return {
    id: record.id,
    threadId: record.threadId,
    subject: record.subject,
    messageCount: record.emailIds.length,
    createdAt: record.createdAt,
    expiresAt: record.expiresAt,
    revokedAt: record.revokedAt || '',
    active: isActive(record, new Date()),
  }
}

/** @param {any} email @param {any[]} attachments */
function presentPublicMessage(email, attachments) {
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
