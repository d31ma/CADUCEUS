import { createHash, randomBytes } from 'node:crypto'
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { join, resolve } from 'node:path'
import { Buffer } from 'node:buffer'
import { Buckets, Collections, collect } from '../../../shared/persistence/Fylo.js'
import { ServerEnvironment } from '../../../shared/runtime/ServerEnvironment.js'
/** @import { RawMessageRecord } from '../../../types' */

/** Durable, tenant-owned storage for exact RFC 822 message sources. */
export class RawMessagesRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * Persist the exact bytes received from SMTP, or the canonical MIME source
   * generated for a locally sent message.
   *
   * @param {string} emailId
   * @param {string} domain
   * @param {string | Uint8Array} source
   * @returns {Promise<RawMessageRecord>}
   */
  async saveForEmail(emailId, domain, source) {
    await this.deleteForEmail(emailId)
    const bytes = typeof source === 'string' ? Buffer.from(source, 'utf8') : Buffer.from(source)
    const id = randomBytes(16).toString('hex')
    const incomingRoot = resolve(
      ServerEnvironment.value('FYLO_INCOMING_ROOT') || join(this.fylo.root, '.caduceus-incoming'),
    )
    await mkdir(incomingRoot, { recursive: true, mode: 0o700 })
    const temporaryRoot = await mkdtemp(join(incomingRoot, 'raw-message-'))
    const sourcePath = join(temporaryRoot, 'message.eml')
    await writeFile(sourcePath, bytes, { mode: 0o600 })

    let bucketId
    try {
      bucketId = await this.fylo.putFile(Buckets.RAW_MESSAGE_FILES, sourcePath, {
        key: `/emails/${createHash('sha256').update(emailId).digest('hex')}/source.eml`,
        maxBytes: bytes.byteLength,
        meta: { domain },
      })
    } finally {
      await rm(temporaryRoot, { recursive: true, force: true })
    }

    /** @type {RawMessageRecord} */
    const record = {
      id,
      emailId,
      domain,
      bucketId,
      size: bytes.byteLength,
      sha256: createHash('sha256').update(bytes).digest('hex'),
      createdAt: new Date().toISOString(),
    }
    try {
      await this.fylo.putData(Collections.RAW_MESSAGES, record)
    } catch (error) {
      await this.fylo.delDoc(Buckets.RAW_MESSAGE_FILES, bucketId).catch(() => {})
      throw error
    }
    return record
  }

  /** @param {string} emailId @returns {Promise<(RawMessageRecord & { docId: string }) | null>} */
  async findForEmail(emailId) {
    const documents = await collect(
      this.fylo.findDocs(Collections.RAW_MESSAGES, { $ops: [{ emailId: { $eq: emailId } }] }).collect(),
    )
    const entry = Object.entries(documents)[0]
    return entry ? { docId: entry[0], .../** @type {RawMessageRecord} */ (entry[1]) } : null
  }

  /** @param {RawMessageRecord} record @returns {Promise<Uint8Array>} */
  async readContent(record) {
    return await this.fylo.getFileBytes(Buckets.RAW_MESSAGE_FILES, record.bucketId)
  }

  /** @param {string} emailId @returns {Promise<void>} */
  async deleteForEmail(emailId) {
    const record = await this.findForEmail(emailId)
    if (!record) return
    await this.fylo.delDoc(Collections.RAW_MESSAGES, record.docId)
    await this.fylo.delDoc(Buckets.RAW_MESSAGE_FILES, record.bucketId)
  }
}
