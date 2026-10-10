import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class SnoozeHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @param {SnoozeEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.SNOOZED, entry)
  }

  /**
   * @returns {Promise<Record<string, SnoozeEntry>>}
   */
  async listElapsed() {
    return await this.listDue(Date.now())
  }

  /**
   * @returns {Promise<Record<string, SnoozeEntry>>}
   */
  async listAll() {
    const fylo = await this.fyloHelper.database()
    return /** @type {Promise<Record<string, SnoozeEntry>>} */ (
      Fylo.collect(fylo.findDocs(Fylo.Collections.SNOOZED, { $ops: [] }).collect())
    )
  }

  /** @param {string} [ownerEmail] @returns {Promise<Record<string, SnoozeEntry>>} */
  async listActive(ownerEmail) {
    const docs = await this.listAll()
    const owner = String(ownerEmail || '').toLowerCase()
    return Object.fromEntries(Object.entries(docs).filter(([, entry]) => {
      const active = !entry.status || entry.status === 'active'
      return active && (!owner || !entry.ownerEmail || entry.ownerEmail.toLowerCase() === owner)
    }))
  }

  /** @param {number} now @param {number} [limit] @returns {Promise<Record<string, SnoozeEntry>>} */
  async listDue(now, limit = 100) {
    const active = await this.listActive()
    return Object.fromEntries(Object.entries(active)
      .filter(([, entry]) => entry.until <= now)
      .sort((left, right) => left[1].until - right[1].until)
      .slice(0, Math.max(1, Math.min(500, limit))))
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, SnoozeEntry | null]>}
   */
  async findById(id) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.SNOOZED, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {SnoozeEntry} */ (entry[1])] : [null, null]
  }

  /** @param {string} ownerEmail @param {string} threadId @returns {Promise<[string | null, SnoozeEntry | null]>} */
  async findActiveForThread(ownerEmail, threadId) {
    const entries = await this.listActive(ownerEmail)
    const match = Object.entries(entries).find(([, entry]) => entry.threadId === threadId)
    return match ? [match[0], /** @type {SnoozeEntry} */ (match[1])] : [null, null]
  }

  /** @param {string} ownerEmail @param {string} clientRequestId @returns {Promise<[string | null, SnoozeEntry | null]>} */
  async findByClientRequestId(ownerEmail, clientRequestId) {
    const owner = ownerEmail.toLowerCase()
    const match = Object.entries(await this.listAll()).find(([, entry]) =>
      entry.ownerEmail?.toLowerCase() === owner && entry.clientRequestId === clientRequestId)
    return match ? [match[0], /** @type {SnoozeEntry} */ (match[1])] : [null, null]
  }

  /** @param {string} docId @param {Partial<SnoozeEntry>} patch */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.SNOOZED, { [docId]: patch })
  }

  /**
   * @param {string} emailId
   * @returns {Promise<void>}
   */
  async removeForEmail(emailId) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.SNOOZED, { $ops: [{ emailId: { $eq: emailId } }] }).collect())
    for (const docId of Object.keys(docs)) {
      await fylo.delDoc(Fylo.Collections.SNOOZED, docId)
    }
  }
}
