import { Collections, collect } from '../../../shared/persistence/Fylo.js'

export class OutboxRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {OutboxEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    return await this.fylo.putData(Collections.OUTBOX, entry)
  }

  /**
   * @param {string} undoId
   * @returns {Promise<[string | null, OutboxEntry | null]>}
   */
  async findById(undoId) {
    const docs = await collect(this.fylo.findDocs(Collections.OUTBOX, { $ops: [{ id: { $eq: undoId } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {OutboxEntry} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    await this.fylo.delDoc(Collections.OUTBOX, docId)
  }

  /**
   * @returns {Promise<Record<string, OutboxEntry>>}
   */
  async findExpired() {
    return /** @type {Promise<Record<string, OutboxEntry>>} */ (
      collect(this.fylo.findDocs(Collections.OUTBOX, { $ops: [{ expiresAt: { $lte: Date.now() } }] }).collect())
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
    const all = /** @type {Record<string, OutboxEntry>} */ (
      await collect(this.fylo.findDocs(Collections.OUTBOX, { $ops: [{ expiresAt: { $lte: now } }] }).collect())
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
    return /** @type {Promise<Record<string, OutboxEntry>>} */ (
      collect(this.fylo.findDocs(Collections.OUTBOX, { $ops: [] }).collect())
    )
  }

  /**
   * @param {string} docId
   * @param {Partial<OutboxEntry>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    await this.fylo.patchDoc(Collections.OUTBOX, { [docId]: patch })
  }
}
