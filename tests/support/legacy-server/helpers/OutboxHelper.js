import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class OutboxHelper {
  /** @type {FyloHelper | undefined} */
  @Needs
  fyloHelper

  /**
   * @param {OutboxEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    return await fylo.putData(Fylo.Collections.OUTBOX, entry)
  }

  /**
   * @param {string} undoId
   * @returns {Promise<[string | null, OutboxEntry | null]>}
   */
  async findById(undoId) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.OUTBOX, { $ops: [{ id: { $eq: undoId } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {OutboxEntry} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    await fylo.delDoc(Fylo.Collections.OUTBOX, docId)
  }

  /**
   * @returns {Promise<Record<string, OutboxEntry>>}
   */
  async findExpired() {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    return /** @type {Promise<Record<string, OutboxEntry>>} */ (
      Fylo.collect(fylo.findDocs(Fylo.Collections.OUTBOX, { $ops: [{ expiresAt: { $lte: Date.now() } }] }).collect())
    )
  }

  /**
   * Entries whose next delivery attempt is due. Falls back to `expiresAt` for
   * entries written before the retry fields existed.
   *
   * @param {number} [now]
   * @returns {Promise<Record<string, OutboxEntry>>}
   */
  async findDue(now = Date.now()) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const all = /** @type {Record<string, OutboxEntry>} */ (
      await Fylo.collect(fylo.findDocs(Fylo.Collections.OUTBOX, { $ops: [{ expiresAt: { $lte: now } }] }).collect())
    )
    /** @type {Record<string, OutboxEntry>} */
    const due = {}
    for (const [docId, entry] of Object.entries(all)) {
      const at = entry.nextAttempt ?? entry.expiresAt
      if (at <= now) due[docId] = entry
    }
    return due
  }

  /** @returns {Promise<Record<string, OutboxEntry>>} */
  async listAll() {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    return /** @type {Promise<Record<string, OutboxEntry>>} */ (
      Fylo.collect(fylo.findDocs(Fylo.Collections.OUTBOX, { $ops: [] }).collect())
    )
  }

  /**
   * @param {string} docId
   * @param {Partial<OutboxEntry>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    await fylo.patchDoc(Fylo.Collections.OUTBOX, { [docId]: patch })
  }
}
