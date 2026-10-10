import { Collections, collect } from '../../../platform/persistence/fylo.js'

/**
 * @typedef {object} OutboxEntry
 * @property {string} id
 * @property {import('../services/smtp').SendRequest} email
 * @property {string} sender
 * @property {string} [owner]
 * @property {number} expiresAt          When the entry first becomes eligible (undo window end).
 * @property {number} [nextAttempt]      Earliest time the next delivery attempt may run.
 * @property {number} [attempts]         Delivery attempts made so far.
 * @property {'queued' | 'sending'} [status]
 * @property {string} [lastError]
 * @property {number} [createdAt]
 * @property {string} [trackingId]
 * @property {string} [scheduledId]      Logical scheduled-send record linked to this delivery.
 * @property {number} [claimedAt]        Worker lease start; stale leases are reclaimable.
 * @property {string} [attachmentEmailId] Bucket-backed attachment set to hydrate before SMTP.
 * @property {string} [archiveStateJson] Undo-safe source conversation folder snapshot.
 */

export class OutboxRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
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
