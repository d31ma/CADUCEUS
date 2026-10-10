import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { basename, join, relative, resolve } from 'node:path'
import { createHash, randomBytes } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { Buckets, Collections, collect } from '../../../platform/persistence/Fylo.js'
import { normalizeAttachmentContentId, sanitizeAttachmentFilename } from '../services/attachment-policy.js'
import { ServerEnvironment } from '../../../platform/runtime/ServerEnvironment.js'
/** @import { ParsedAttachment } from '../../../models/ParsedAttachment.js' */
/** @import { EmailAttachmentRecord } from '../../../types' */
/** @import { EmailAttachmentSummary } from '../../../types' */

export class AttachmentsRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {string} emailId
   * @returns {Promise<Array<EmailAttachmentRecord & { docId: string }>>}
   */
  async list(emailId) {
    const docs = await collect(this.fylo.findDocs(Collections.ATTACHMENTS, { $ops: [{ emailId: { $eq: emailId } }] }).collect())
    return Object.entries(docs).map(([docId, attachment]) => ({ docId, .../** @type {EmailAttachmentRecord} */ (attachment) }))
  }

  /**
   * @param {string} emailId
   * @returns {Promise<EmailAttachmentSummary[]>}
   */
  async listSummaries(emailId) {
    return (await this.list(emailId)).map(toSummary)
  }

  /**
   * @param {string[]} allowedDomains
   * @returns {Promise<Map<string, EmailAttachmentSummary[]>>}
   */
  async listSummariesByEmail(allowedDomains) {
    const docs = await collect(this.fylo.findDocs(Collections.ATTACHMENTS, { $ops: [] }).collect())
    const grouped = new Map()
    for (const attachment of Object.values(docs)) {
      const record = /** @type {EmailAttachmentRecord} */ (attachment)
      if (!allowedDomains.includes(record.domain)) continue
      const summaries = grouped.get(record.emailId) ?? []
      summaries.push(toSummary(record))
      grouped.set(record.emailId, summaries)
    }
    return grouped
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, EmailAttachmentRecord | null]>}
   */
  async findById(id) {
    const docs = await collect(this.fylo.findDocs(Collections.ATTACHMENTS, { $ops: [{ id: { $eq: id } }] }).collect())
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
      const record = await persistOne(this.fylo, emailId, domain, attachment)
      saved.push(toSummary(record))
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
      for (const attachment of attachments) saved.push(await persistOne(this.fylo, emailId, domain, attachment))
    } catch (error) {
      await Promise.all(saved.map(record => deleteRecord(this.fylo, record).catch(() => {})))
      throw error
    }
    await Promise.all(previous.map(record => deleteRecord(this.fylo, record)))
    return saved.map(toSummary)
  }

  /**
   * @param {string} emailId
   * @returns {Promise<void>}
   */
  async deleteForEmail(emailId) {
    const attachments = await this.list(emailId)
    await Promise.all(attachments.map(record => deleteRecord(this.fylo, record)))
  }

  /**
   * @param {EmailAttachmentRecord} attachment
   * @returns {Promise<Uint8Array>}
   */
  async readContent(attachment) {
    if (attachment.bucketId) {
      return await this.fylo.getFileBytes(Buckets.ATTACHMENT_FILES, attachment.bucketId)
    }
    if (!attachment.storagePath) throw new Error('Attachment has no stored content reference')
    ensureInsideRoot(attachmentRoot(), attachment.storagePath)
    return await Bun.file(attachment.storagePath).bytes()
  }
}

/** @param {EmailAttachmentRecord} attachment @returns {EmailAttachmentSummary} */
function toSummary({ id, filename, contentType, size, disposition, contentId }) {
  return { id, filename, contentType, size, disposition, contentId }
}

function attachmentRoot() {
  // Legacy path-backed records may point at the former dedicated mount.
  if (ServerEnvironment.value('ATTACHMENT_ROOT')) return ServerEnvironment.value('ATTACHMENT_ROOT')
  if (ServerEnvironment.value('FYLO_ROOT')) return join(ServerEnvironment.value('FYLO_ROOT'), 'attachments')
  throw new Error('FYLO_ROOT or legacy ATTACHMENT_ROOT is required to read a path-backed attachment')
}

/**
 * @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo
 * @param {string} emailId
 * @param {string} domain
 * @param {ParsedAttachment} attachment
 * @returns {Promise<EmailAttachmentRecord & { docId: string }>}
 */
async function persistOne(fylo, emailId, domain, attachment) {
  const id = randomBytes(16).toString('hex')
  const filename = sanitizeAttachmentFilename(attachment.filename, `attachment-${id}`)
  const content = readBytes(attachment)
  const incomingRoot = resolve(
    ServerEnvironment.value('FYLO_INCOMING_ROOT') || join(fylo.root, '.caduceus-incoming'),
  )
  await mkdir(incomingRoot, { recursive: true, mode: 0o700 })
  const temporaryRoot = await mkdtemp(join(incomingRoot, 'attachment-'))
  const sourcePath = join(temporaryRoot, filename)
  await writeFile(sourcePath, content)
  let bucketId
  try {
    bucketId = await fylo.putFile(Buckets.ATTACHMENT_FILES, sourcePath, {
      key: bucketKey(emailId, id, filename),
      maxBytes: content.byteLength,
      meta: { domain },
    })
  } finally {
    await rm(temporaryRoot, { recursive: true, force: true })
  }
  /** @type {EmailAttachmentRecord} */
  const record = {
    id,
    emailId,
    domain,
    filename,
    contentType: String(attachment.contentType || 'application/octet-stream').split(';', 1)[0].toLowerCase(),
    size: content.byteLength,
    disposition: attachment.disposition || 'attachment',
    contentId: normalizeAttachmentContentId(attachment.contentId),
    bucketId,
    createdAt: new Date().toISOString(),
  }
  try {
    const docId = await fylo.putData(Collections.ATTACHMENTS, record)
    return { ...record, docId }
  } catch (error) {
    await fylo.delDoc(Buckets.ATTACHMENT_FILES, bucketId).catch(() => {})
    throw error
  }
}

/** @param {string} emailId @param {string} id @param {string} filename */
function bucketKey(emailId, id, filename) {
  const emailKey = createHash('sha256').update(emailId).digest('hex')
  return `/emails/${emailKey}/${id}-${basename(filename)}`
}

/** @param {string} name */
/** @param {string} root @param {string} target */
function ensureInsideRoot(root, target) {
  const resRoot = resolve(root)
  const resTarget = resolve(target)
  const rel = relative(resRoot, resTarget)
  if (rel.startsWith('..') || rel === '..' || rel.startsWith('/')) {
    throw new Error('Attachment path escapes storage root')
  }
}

/** @param {ParsedAttachment} attachment @returns {Uint8Array} */
function readBytes(attachment) {
  if (attachment.content instanceof Uint8Array) return attachment.content
  if (typeof attachment.contentBase64 === 'string') return Buffer.from(attachment.contentBase64, 'base64')
  return new Uint8Array()
}

/** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo @param {EmailAttachmentRecord & { docId?: string }} record */
async function deleteRecord(fylo, { docId, bucketId, storagePath }) {
  if (!docId) throw new Error('Attachment record is missing its document id')
  await fylo.delDoc(Collections.ATTACHMENTS, docId)
  if (bucketId) {
    await fylo.delDoc(Buckets.ATTACHMENT_FILES, bucketId)
  } else if (storagePath) {
    try {
      ensureInsideRoot(attachmentRoot(), storagePath)
      await rm(storagePath, { force: true })
    } catch (error) {
      console.error('[attachments] delete legacy attachment file failed:', error)
    }
  }
}
