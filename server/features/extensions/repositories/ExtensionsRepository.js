import { Fylo } from '../../../shared/persistence/Fylo.js'

export class ExtensionsRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {ExtensionRecord} record */
  async put(record) { return await this.fylo.putData(Fylo.Collections.EXTENSIONS, record) }

  /** @returns {Promise<Array<ExtensionRecord & {docId: string}>>} */
  async listForOwner(owner) {
    const documents = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.EXTENSIONS, {
      $ops: [{ owner: { $eq: String(owner).toLowerCase() } }],
    }).collect())
    return Object.entries(documents).map(([docId, record]) => ({
      docId,
      .../** @type {ExtensionRecord} */ (record),
    }))
  }

  async findForOwner(owner, extensionId) {
    const records = await this.listForOwner(owner)
    return records.find(record => record.extensionId === extensionId) ?? null
  }

  async update(docId, patch) { await this.fylo.patchDoc(Fylo.Collections.EXTENSIONS, { [docId]: patch }) }
  async delete(docId) { await this.fylo.delDoc(Fylo.Collections.EXTENSIONS, docId) }
}

