import { mkdir, mkdtemp, rm, writeFile, readFile } from 'node:fs/promises'
import { basename, join, relative, resolve } from 'node:path'
import { createHash, randomBytes } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { Fylo } from '../persistence/Fylo.js'
import { AttachmentPolicy } from './AttachmentPolicy.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

export class AttachmentsRecords {
  /** @param {EmailAttachmentRecord} attachment @returns {EmailAttachmentSummary} */
  static toSummary({ id, filename, contentType, size, disposition, contentId }) {
    return { id, filename, contentType, size, disposition, contentId }
  }

  static attachmentRoot() {
    // Legacy path-backed records may point at the former dedicated mount.
    if (ServerEnvironment.value('ATTACHMENT_ROOT')) return ServerEnvironment.value('ATTACHMENT_ROOT')
    if (ServerEnvironment.value('FYLO_ROOT')) return join(ServerEnvironment.value('FYLO_ROOT'), 'attachments')
    throw new Error('FYLO_ROOT or legacy ATTACHMENT_ROOT is required to read a path-backed attachment')
  }

  /**
   * @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo
   * @param {string} emailId
   * @param {string} domain
   * @param {ParsedAttachment} attachment
   * @returns {Promise<EmailAttachmentRecord & { docId: string }>}
   */
  static async persistOne(fylo, emailId, domain, attachment) {
    const id = randomBytes(16).toString('hex')
    const filename = AttachmentPolicy.sanitizeAttachmentFilename(attachment.filename, `attachment-${id}`)
    const content = AttachmentsRecords.readBytes(attachment)
    const incomingRoot = resolve(
      ServerEnvironment.value('FYLO_INCOMING_ROOT') || join(fylo.root, '.caduceus-incoming'),
    )
    await mkdir(incomingRoot, { recursive: true, mode: 0o700 })
    const temporaryRoot = await mkdtemp(join(incomingRoot, 'attachment-'))
    const sourcePath = join(temporaryRoot, filename)
    await writeFile(sourcePath, content)
    let bucketId
    try {
      bucketId = await fylo.putFile(Fylo.Buckets.ATTACHMENT_FILES, sourcePath, {
        key: AttachmentsRecords.bucketKey(emailId, id, filename),
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
      contentId: AttachmentPolicy.normalizeAttachmentContentId(attachment.contentId),
      bucketId,
      createdAt: new Date().toISOString(),
    }
    try {
      const docId = await fylo.putData(Fylo.Collections.ATTACHMENTS, record)
      return { ...record, docId }
    } catch (error) {
      await fylo.delDoc(Fylo.Buckets.ATTACHMENT_FILES, bucketId).catch(() => {})
      throw error
    }
  }

  /** @param {string} emailId @param {string} id @param {string} filename */
  static bucketKey(emailId, id, filename) {
    const emailKey = createHash('sha256').update(emailId).digest('hex')
    return `/emails/${emailKey}/${id}-${basename(filename)}`
  }

  /** @param {string} name */
  /** @param {string} root @param {string} target */
  static ensureInsideRoot(root, target) {
    const resRoot = resolve(root)
    const resTarget = resolve(target)
    const rel = relative(resRoot, resTarget)
    if (rel.startsWith('..') || rel === '..' || rel.startsWith('/')) {
      throw new Error('Attachment path escapes storage root')
    }
  }

  /** @param {ParsedAttachment} attachment @returns {Uint8Array} */
  static readBytes(attachment) {
    if (attachment.content instanceof Uint8Array) return attachment.content
    if (typeof attachment.contentBase64 === 'string') return Buffer.from(attachment.contentBase64, 'base64')
    return new Uint8Array()
  }

  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo @param {EmailAttachmentRecord & { docId?: string }} record */
  static async deleteRecord(fylo, { docId, bucketId, storagePath }) {
    if (!docId) throw new Error('Attachment record is missing its document id')
    await fylo.delDoc(Fylo.Collections.ATTACHMENTS, docId)
    if (bucketId) {
      await fylo.delDoc(Fylo.Buckets.ATTACHMENT_FILES, bucketId)
    } else if (storagePath) {
      try {
        AttachmentsRecords.ensureInsideRoot(AttachmentsRecords.attachmentRoot(), storagePath)
        await rm(storagePath, { force: true })
      } catch (error) {
        console.error('[attachments] delete legacy attachment file failed:', error)
      }
    }
  }

  /**
   * @param {string} path
   * @returns {Promise<Uint8Array>}
   */
  static async readFileBytes(path) {
    return new Uint8Array(await readFile(path))
  }
}
