import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class FollowUpRemindersHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /** @param {FollowUpReminder} reminder */
  async put(reminder) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.FOLLOW_UP_REMINDERS, reminder)
  }

  /** @param {string} ownerEmail @returns {Promise<Array<FollowUpReminder & { docId: string }>>} */
  async listForOwner(ownerEmail) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.FOLLOW_UP_REMINDERS, {
      $ops: [{ ownerEmail: { $eq: ownerEmail } }],
    }).collect())
    return Object.entries(docs).map(([docId, reminder]) => ({
      .../** @type {FollowUpReminder} */ (reminder), docId,
    }))
  }

  /** @param {string} ownerEmail */
  async listPending(ownerEmail) {
    return (await this.listForOwner(ownerEmail)).filter(item => item.status === 'scheduled')
  }

  /** Return every scheduled reminder for the system-owned due worker. */
  async listPendingAll() {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.FOLLOW_UP_REMINDERS, {
      $ops: [{ status: { $eq: 'scheduled' } }],
    }).collect())
    return Object.entries(docs).map(([docId, reminder]) => ({
      .../** @type {FollowUpReminder} */ (reminder), docId,
    }))
  }

  /** @param {string} id @returns {Promise<[string|null, FollowUpReminder|null]>} */
  async findById(id) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.FOLLOW_UP_REMINDERS, {
      $ops: [{ id: { $eq: id } }],
    }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {FollowUpReminder} */ (entry[1])] : [null, null]
  }

  /** @param {string} ownerEmail @param {string} emailId */
  async findActiveForEmail(ownerEmail, emailId) {
    return (await this.listForOwner(ownerEmail)).find(
      item => item.emailId === emailId && (item.status === 'scheduled' || item.status === 'due'),
    ) ?? null
  }

  /** @param {string} docId @param {Partial<FollowUpReminder>} patch */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.FOLLOW_UP_REMINDERS, { [docId]: patch })
  }
}
