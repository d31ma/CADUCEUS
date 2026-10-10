import { Collections, collect } from '../../../platform/persistence/fylo.js'

export class ExtensionsRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {ExtensionRecord} record */
  async put(record) { return await this.fylo.putData(Collections.EXTENSIONS, record) }

  /** @returns {Promise<Array<ExtensionRecord & {docId: string}>>} */
  async listForOwner(owner) {
    const documents = await collect(this.fylo.findDocs(Collections.EXTENSIONS, {
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

  async update(docId, patch) { await this.fylo.patchDoc(Collections.EXTENSIONS, { [docId]: patch }) }
  async delete(docId) { await this.fylo.delDoc(Collections.EXTENSIONS, docId) }
}

/**
 * @typedef {object} ExtensionRecord
 * @property {string} id
 * @property {string} extensionId
 * @property {string} owner
 * @property {string} domain
 * @property {'plugin'|'theme'} kind
 * @property {string} displayName
 * @property {string} version
 * @property {string} digest
 * @property {string} manifestJson
 * @property {string} source
 * @property {boolean} enabled
 * @property {string} trustedAt
 * @property {string} installedAt
 * @property {string} updatedAt
 */
