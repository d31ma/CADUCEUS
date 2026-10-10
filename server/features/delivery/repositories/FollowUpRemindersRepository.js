import { Fylo } from '../../../shared/persistence/Fylo.js'

export class FollowUpRemindersRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {FollowUpReminder} reminder */
  async put(reminder) {
    return await this.fylo.putData(Fylo.Collections.FOLLOW_UP_REMINDERS, reminder)
  }

  /** @param {string} ownerEmail @returns {Promise<Array<FollowUpReminder & { docId: string }>>} */
  async listForOwner(ownerEmail) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.FOLLOW_UP_REMINDERS, {
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
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.FOLLOW_UP_REMINDERS, {
      $ops: [{ status: { $eq: 'scheduled' } }],
    }).collect())
    return Object.entries(docs).map(([docId, reminder]) => ({
      .../** @type {FollowUpReminder} */ (reminder), docId,
    }))
  }

  /** @param {string} id @returns {Promise<[string|null, FollowUpReminder|null]>} */
  async findById(id) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.FOLLOW_UP_REMINDERS, {
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
    await this.fylo.patchDoc(Fylo.Collections.FOLLOW_UP_REMINDERS, { [docId]: patch })
  }
}
