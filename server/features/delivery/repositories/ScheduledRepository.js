import { Fylo } from '../../../shared/persistence/Fylo.js'

export class ScheduledRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {ScheduledEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    return await this.fylo.putData(Fylo.Collections.SCHEDULED, entry)
  }

  /**
   * @param {string} owner
   * @returns {Promise<Record<string, ScheduledEntry>>}
   */
  async listForOwner(owner) {
    const docs = await this.listAll()
    return /** @type {Record<string, ScheduledEntry>} */ (Object.fromEntries(
      Object.entries(docs).filter(([, entry]) => (entry.owner ?? entry.sender) === owner),
    ))
  }

  /** System-owned worker scan used to reconcile pre-queue scheduled records. */
  async listAll() {
    return /** @type {Promise<Record<string, ScheduledEntry>>} */ (
      Fylo.collect(this.fylo.findDocs(Fylo.Collections.SCHEDULED, { $ops: [] }).collect())
    )
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, ScheduledEntry | null]>}
   */
  async findById(id) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.SCHEDULED, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {ScheduledEntry} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    await this.fylo.delDoc(Fylo.Collections.SCHEDULED, docId)
  }

  /** @param {string} docId @param {Partial<ScheduledEntry>} patch */
  async update(docId, patch) {
    await this.fylo.patchDoc(Fylo.Collections.SCHEDULED, { [docId]: patch })
  }
}
