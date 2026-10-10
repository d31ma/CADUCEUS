import { Collections, collect } from '../../../platform/persistence/Fylo.js'

export class SendRequestsRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {SendRequestRecord} record */
  async put(record) { return await this.fylo.putData(Collections.SEND_REQUESTS, record) }

  /** @param {string} ownerEmail @param {string} idempotencyKey */
  async find(ownerEmail, idempotencyKey) {
    // FYLO combines `$ops` with OR semantics. Narrow the storage query by the
    // trusted owner, then enforce the idempotency key in-process so one send
    // can never be mistaken for another request from the same mailbox.
    const documents = await collect(this.fylo.findDocs(Collections.SEND_REQUESTS, { $ops: [
      { ownerEmail: { $eq: ownerEmail.toLowerCase() } },
    ] }).collect())
    const entry = Object.entries(documents).find(([, record]) =>
      /** @type {SendRequestRecord} */ (record).idempotencyKey === idempotencyKey)
    return entry ? [entry[0], /** @type {SendRequestRecord} */ (entry[1])] : [null, null]
  }

  /** @param {string} documentId @param {Partial<SendRequestRecord>} patch */
  async update(documentId, patch) {
    await this.fylo.patchDoc(Collections.SEND_REQUESTS, { [documentId]: patch })
  }

  /** @param {string} documentId */
  async delete(documentId) { await this.fylo.delDoc(Collections.SEND_REQUESTS, documentId) }
}
