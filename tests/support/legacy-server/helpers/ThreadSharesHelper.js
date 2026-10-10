import { Fylo } from '../shared/persistence/Fylo.js'
import { ThreadSharesRecords } from '../shared/mailbox/ThreadSharesRecords.js'

/** FYLO persistence for mailbox-owned conversation capabilities. */
@Helper
export default class ThreadSharesHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /** @param {ThreadShareRecord} share */
  async put(share) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.THREAD_SHARES, ThreadSharesRecords.serialize(share))
  }

  /** @param {string} ownerEmail @returns {Promise<Array<ThreadShareRecord & { docId: string }>>} */
  async listForOwner(ownerEmail) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.THREAD_SHARES, {
      $ops: [{ ownerEmail: { $eq: ownerEmail.toLowerCase() } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({ docId, ...ThreadSharesRecords.deserialize(value) }))
  }

  /** @param {string} id @returns {Promise<[string | null, ThreadShareRecord | null]>} */
  async findById(id) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.THREAD_SHARES, {
      $ops: [{ id: { $eq: id } }],
    }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], ThreadSharesRecords.deserialize(entry[1])] : [null, null]
  }

  /** @param {string} tokenHash @returns {Promise<Array<ThreadShareRecord & { docId: string }>>} */
  async findByTokenHash(tokenHash) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.THREAD_SHARES, {
      $ops: [{ tokenHash: { $eq: tokenHash } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({ docId, ...ThreadSharesRecords.deserialize(value) }))
  }

  /** @param {string} docId @param {Partial<ThreadShareRecord>} patch */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    const stored = /** @type {Record<string, any>} */ ({ ...patch })
    if (patch.emailIds) {
      stored.emailIdsJson = JSON.stringify(patch.emailIds)
      delete stored.emailIds
    }
    await fylo.patchDoc(Fylo.Collections.THREAD_SHARES, { [docId]: stored })
  }
}

