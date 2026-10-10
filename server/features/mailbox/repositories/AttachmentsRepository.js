import { mkdir, mkdtemp, rm, writeFile, readFile } from 'node:fs/promises'
import { basename, join, relative, resolve } from 'node:path'
import { createHash, randomBytes } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { Fylo } from '../../../shared/persistence/Fylo.js'
import { AttachmentPolicy } from '../../../shared/mailbox/AttachmentPolicy.js'
import { ServerEnvironment } from '../../../shared/runtime/ServerEnvironment.js'
import { AttachmentsRecords } from '../../../shared/mailbox/AttachmentsRecords.js'

/** @import { ParsedAttachment } from '../../../models/ParsedAttachment.js' */
/** @import { EmailAttachmentRecord } from '../../../types' */
/** @import { EmailAttachmentSummary } from '../../../types' */

export class AttachmentsRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {string} emailId
   * @returns {Promise<Array<EmailAttachmentRecord & { docId: string }>>}
   */
  async list(emailId) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.ATTACHMENTS, { $ops: [{ emailId: { $eq: emailId } }] }).collect())
    return Object.entries(docs).map(([docId, attachment]) => ({ docId, .../** @type {EmailAttachmentRecord} */ (attachment) }))
  }

  /**
   * @param {string} emailId
   * @returns {Promise<EmailAttachmentSummary[]>}
   */
  async listSummaries(emailId) {
    return (await this.list(emailId)).map(AttachmentsRecords.toSummary)
  }

  /**
   * @param {string[]} allowedDomains
   * @returns {Promise<Map<string, EmailAttachmentSummary[]>>}
   */
  async listSummariesByEmail(allowedDomains) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.ATTACHMENTS, { $ops: [] }).collect())
    const grouped = new Map()
    for (const attachment of Object.values(docs)) {
      const record = /** @type {EmailAttachmentRecord} */ (attachment)
      if (!allowedDomains.includes(record.domain)) continue
      const summaries = grouped.get(record.emailId) ?? []
      summaries.push(AttachmentsRecords.toSummary(record))
      grouped.set(record.emailId, summaries)
    }
    return grouped
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, EmailAttachmentRecord | null]>}
   */
  async findById(id) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.ATTACHMENTS, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {EmailAttachmentRecord} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} emailId
   * @param {string} domain
   * @param {ParsedAttachment[]} attachments
   * @returns {Promise<EmailAttachmentSummary[]>}
   */
  async saveForEmail(emailId, domain, attachments) {
    if (attachments.length === 0) return []
    const saved = []
    for (const attachment of attachments) {
      const record = await AttachmentsRecords.persistOne(this.fylo, emailId, domain, attachment)
      saved.push(AttachmentsRecords.toSummary(record))
    }
    return saved
  }

  /**
   * Replace a draft's complete attachment set without deleting the old set
   * until every replacement file has been written successfully.
   *
   * @param {string} emailId
   * @param {string} domain
   * @param {ParsedAttachment[]} attachments
   */
  async replaceForEmail(emailId, domain, attachments) {
    const previous = await this.list(emailId)
    /** @type {Array<EmailAttachmentRecord & { docId?: string }>} */
    const saved = []
    try {
      for (const attachment of attachments) saved.push(await AttachmentsRecords.persistOne(this.fylo, emailId, domain, attachment))
    } catch (error) {
      await Promise.all(saved.map(record => AttachmentsRecords.deleteRecord(this.fylo, record).catch(() => {})))
      throw error
    }
    await Promise.all(previous.map(record => AttachmentsRecords.deleteRecord(this.fylo, record)))
    return saved.map(AttachmentsRecords.toSummary)
  }

  /**
   * @param {string} emailId
   * @returns {Promise<void>}
   */
  async deleteForEmail(emailId) {
    const attachments = await this.list(emailId)
    await Promise.all(attachments.map(record => AttachmentsRecords.deleteRecord(this.fylo, record)))
  }

  /**
   * @param {EmailAttachmentRecord} attachment
   * @returns {Promise<Uint8Array>}
   */
  async readContent(attachment) {
    if (attachment.bucketId) {
      return await this.fylo.getFileBytes(Fylo.Buckets.ATTACHMENT_FILES, attachment.bucketId)
    }
    if (!attachment.storagePath) throw new Error('Attachment has no stored content reference')
    AttachmentsRecords.ensureInsideRoot(AttachmentsRecords.attachmentRoot(), attachment.storagePath)
    return new Uint8Array(await readFile(attachment.storagePath))
  }
}

