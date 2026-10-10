import { Collections, collect } from '../../../platform/persistence/fylo.js'

export class MailActionsRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /** @param {MailActionEntry} entry */
  async put(entry) {
    return await this.fylo.putData(Collections.MAIL_ACTIONS, entry)
  }

  /** @param {string} id @returns {Promise<[string | null, MailActionEntry | null]>} */
  async findById(id, ownerEmail = '') {
    if (typeof id !== 'string' || !id) return [null, null]
    const documents = await this.fylo.getDoc(Collections.MAIL_ACTIONS, id)
    const entry = documents?.[id] ?? documents?.identifier ?? Object.values(documents ?? {})[0]
    if (entry && typeof entry === 'object' && (!ownerEmail || entry.ownerEmail?.toLowerCase() === ownerEmail.toLowerCase())) {
      return [id, /** @type {MailActionEntry} */ (entry)]
    }

    // Historical FYLO roots may have used a logical id distinct from the
    // document identifier. Keep a bounded logical-id fallback; authorization
    // is still enforced against ownerEmail by the service.
    const matches = await collect(this.fylo.findDocs(Collections.MAIL_ACTIONS, { $ops: [{ id: { $eq: id } }] }).collect())
    const found = Object.entries(matches).find(([docId, record]) =>
      (docId === id || record?.id === id) && (!ownerEmail || record?.ownerEmail?.toLowerCase() === ownerEmail.toLowerCase()))
    return found && found[1] && typeof found[1] === 'object'
      ? [found[0], /** @type {MailActionEntry} */ (found[1])]
      : [null, null]
  }

  /** @param {string} ownerEmail @returns {Promise<Array<[string, MailActionEntry]>>} */
  async listForOwner(ownerEmail) {
    const docs = await collect(
      this.fylo.findDocs(Collections.MAIL_ACTIONS, { $ops: [{ ownerEmail: { $eq: ownerEmail } }] }).collect(),
    )
    const owner = ownerEmail.toLowerCase()
    return /** @type {Array<[string, MailActionEntry]>} */ (
      Object.entries(docs).filter(([, entry]) => entry.ownerEmail?.toLowerCase() === owner)
    )
  }

  /** @param {string} ownerEmail @param {string} clientRequestId */
  async findByClientRequestId(ownerEmail, clientRequestId) {
    const entry = (await this.listForOwner(ownerEmail))
      .find(([, record]) => record.clientRequestId === clientRequestId)
    return entry || [null, null]
  }

  /** @param {string} docId @param {Partial<MailActionEntry>} patch */
  async update(docId, patch) {
    await this.fylo.patchDoc(Collections.MAIL_ACTIONS, { [docId]: patch })
  }

  /** Re-resolve the latest FYLO document before every journal transition. */
  async updateById(id, patch, ownerEmail = '') {
    const [docId, record] = await this.findById(id, ownerEmail)
    if (!docId || !record) return false
    await this.update(docId, patch)
    return true
  }
}
