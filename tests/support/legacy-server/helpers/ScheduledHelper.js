import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class ScheduledHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @param {ScheduledEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.SCHEDULED, entry)
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
    const fylo = await this.fyloHelper.database()
    return /** @type {Promise<Record<string, ScheduledEntry>>} */ (
      Fylo.collect(fylo.findDocs(Fylo.Collections.SCHEDULED, { $ops: [] }).collect())
    )
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, ScheduledEntry | null]>}
   */
  async findById(id) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.SCHEDULED, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {ScheduledEntry} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    const fylo = await this.fyloHelper.database()
    await fylo.delDoc(Fylo.Collections.SCHEDULED, docId)
  }

  /** @param {string} docId @param {Partial<ScheduledEntry>} patch */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.SCHEDULED, { [docId]: patch })
  }
}
