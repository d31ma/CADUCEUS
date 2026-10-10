import { EmailsRecords } from '../shared/mailbox/EmailsRecords.js'
import { FyloAccessContext } from '../shared/persistence/FyloAccessContext.js'
import { Security } from '../shared/security/Security.js'
import { ThreadSharingRules } from '../shared/mailbox/ThreadSharingRules.js'
import { ThreadShareInputError } from '../shared/errors/ThreadShareInputError.js'

/**
 * Creates and resolves self-hosted conversation-sharing capabilities.
 * The bearer secret is returned once and never persisted; FYLO stores only a
 * SHA-256 hash, exact mailbox ownership, immutable message membership, expiry,
 * and revocation state.
 */
@Service
export default class ThreadSharingService {
  /** @type {ThreadSharesHelper} */
  @Needs
  threadSharesHelper

  /** @type {ThreadsHelper} */
  @Needs
  threadsHelper

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /**
   * @param {{ ownerEmail: string, domains: string[], emailId: string, expiresInDays: number }} input
   */
  async create(input) {
    const expiresInDays = Number(input.expiresInDays)
    if (!Number.isInteger(expiresInDays) || !ThreadSharingRules.EXPIRY_DAYS.has(expiresInDays)) {
      throw new ThreadShareInputError('expiresInDays must be one of: 1, 7, 30')
    }
    const emailId = String(input.emailId || '').trim()
    if (!emailId || emailId.length > 500) throw new ThreadShareInputError('emailId is required')

    const thread = await this.threadsHelper.findForEmail(input.domains, emailId, input.ownerEmail)
    if (!thread) return null
    const domain = Security.normalizeDomain(thread.latestEmail.domain)
    if (!domain || !input.domains.includes(domain)) return null

    const emailIds = thread.emails.map(email => email.id)
    const serializedIds = JSON.stringify(emailIds)
    if (Buffer.byteLength(serializedIds) > ThreadSharingRules.MAX_EMAIL_IDS_JSON_BYTES) {
      throw new ThreadShareInputError('This conversation is too large to share safely')
    }

    const now = new Date()
    const revokedAt = now.toISOString()
    const existing = await this.threadSharesHelper.listForOwner(input.ownerEmail)
    await Promise.all(existing
      .filter(share => share.threadId === thread.id && ThreadSharingRules.isActive(share, now))
      .map(share => this.threadSharesHelper.update(share.docId, { revokedAt })))

    const token = ThreadSharingRules.createToken(domain)
    const record = {
      id: `share-${Security.randomText(16, 'base64url')}`,
      ownerEmail: input.ownerEmail.toLowerCase(),
      domain,
      threadId: thread.id,
      subject: String(thread.subject || '(No subject)').slice(0, 500),
      emailIds,
      tokenHash: ThreadSharingRules.hashToken(token),
      createdAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + expiresInDays * 86_400_000).toISOString(),
    }
    await this.threadSharesHelper.put(record)
    return { ...ThreadSharingRules.presentManaged(record), token, url: `/share/${encodeURIComponent(token)}` }
  }

  /** @param {string} ownerEmail @param {string} [emailId] @param {string[]} [domains] */
  async list(ownerEmail, emailId, domains = []) {
    const records = await this.threadSharesHelper.listForOwner(ownerEmail)
    let selected = records
    if (emailId) {
      const thread = await this.threadsHelper.findForEmail(domains, emailId, ownerEmail)
      if (!thread) return null
      selected = records.filter(share => share.threadId === thread.id)
    }
    return selected
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
      .map(ThreadSharingRules.presentManaged)
  }

  /** @param {string} id @param {string} ownerEmail */
  async revoke(id, ownerEmail) {
    const [docId, record] = await this.threadSharesHelper.findById(id)
    if (!docId || !record || record.ownerEmail !== ownerEmail.toLowerCase()) return false
    if (!record.revokedAt) await this.threadSharesHelper.update(docId, { revokedAt: new Date().toISOString() })
    return true
  }

  /** Resolve and re-authorize every message each time a public capability is used. */
  async resolve(token) {
    const parsed = ThreadSharingRules.parseToken(token)
    if (!parsed) return null
    return await FyloAccessContext.runForDomain(parsed.domain, async () => {
      const records = await this.threadSharesHelper.findByTokenHash(parsed.hash)
      const record = records.find(candidate =>
        candidate.domain === parsed.domain && ThreadSharingRules.equalHash(candidate.tokenHash, parsed.hash))
      if (!record || !ThreadSharingRules.isActive(record, new Date())) return null

      const messages = []
      for (const emailId of record.emailIds) {
        const [, email] = await this.emailsHelper.findById(emailId)
        if (!email || email.domain !== record.domain || !EmailsRecords.canAccessEmail(email, record.ownerEmail)) continue
        const attachments = await this.attachmentsHelper.listSummaries(email.id)
        messages.push(ThreadSharingRules.presentPublicMessage(email, attachments))
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
    const parsed = ThreadSharingRules.parseToken(token)
    if (!parsed) return null
    return await FyloAccessContext.runForDomain(parsed.domain, async () => {
      const records = await this.threadSharesHelper.findByTokenHash(parsed.hash)
      const record = records.find(candidate =>
        candidate.domain === parsed.domain && ThreadSharingRules.equalHash(candidate.tokenHash, parsed.hash))
      if (!record || !ThreadSharingRules.isActive(record, new Date())) return null

      const [, attachment] = await this.attachmentsHelper.findById(attachmentId)
      if (!attachment || attachment.domain !== record.domain || !record.emailIds.includes(attachment.emailId)) return null
      const [, email] = await this.emailsHelper.findById(attachment.emailId)
      if (!email || !EmailsRecords.canAccessEmail(email, record.ownerEmail)) return null
      const bytes = await this.attachmentsHelper.readContent(attachment)
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

