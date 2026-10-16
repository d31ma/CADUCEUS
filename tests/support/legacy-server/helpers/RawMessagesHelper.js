import { Fylo } from '../shared/persistence/Fylo.js'
import { RawMessagesRecords } from '../shared/mailbox/RawMessagesRecords.js'
import { Security } from '../shared/security/Security.js'
/** @import { RawMessageRecord } from '../types' */

/** Durable, tenant-owned storage for exact RFC 822 message sources. */
@Helper
export default class RawMessagesHelper {
  /** @type {FyloHelper | undefined} */
  @Needs
  fyloHelper

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
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    await this.deleteForEmail(emailId)
    const bytes = typeof source === 'string' ? Buffer.from(source, 'utf8') : Buffer.from(source)
    const id = Security.randomText(16)
    const bucketId = await RawMessagesRecords.withStagedSource(fylo.root, bytes, sourcePath =>
      fylo.putFile(Fylo.Buckets.RAW_MESSAGE_FILES, sourcePath, {
        key: `/emails/${Security.sha256Hex(emailId)}/source.eml`,
        maxBytes: bytes.byteLength,
        meta: { domain },
      }))

    /** @type {RawMessageRecord} */
    const record = {
      id,
      emailId,
      domain,
      bucketId,
      size: bytes.byteLength,
      sha256: Security.sha256Hex(bytes),
      createdAt: new Date().toISOString(),
    }
    try {
      await fylo.putData(Fylo.Collections.RAW_MESSAGES, record)
    } catch (error) {
      await fylo.delDoc(Fylo.Buckets.RAW_MESSAGE_FILES, bucketId).catch(() => {})
      throw error
    }
    return record
  }

  /** @param {string} emailId @returns {Promise<(RawMessageRecord & { docId: string }) | null>} */
  async findForEmail(emailId) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const documents = await Fylo.collect(
      fylo.findDocs(Fylo.Collections.RAW_MESSAGES, { $ops: [{ emailId: { $eq: emailId } }] }).collect(),
    )
    const entry = Object.entries(documents)[0]
    return entry ? { docId: entry[0], .../** @type {RawMessageRecord} */ (entry[1]) } : null
  }

  /** @param {RawMessageRecord} record @returns {Promise<Uint8Array>} */
  async readContent(record) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    return await fylo.getFileBytes(Fylo.Buckets.RAW_MESSAGE_FILES, record.bucketId)
  }

  /** @param {string} emailId @returns {Promise<void>} */
  async deleteForEmail(emailId) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const record = await this.findForEmail(emailId)
    if (!record) return
    await fylo.delDoc(Fylo.Collections.RAW_MESSAGES, record.docId)
    await fylo.delDoc(Fylo.Buckets.RAW_MESSAGE_FILES, record.bucketId)
  }
}
