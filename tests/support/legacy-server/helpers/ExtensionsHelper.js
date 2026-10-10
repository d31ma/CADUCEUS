import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class ExtensionsHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /** @param {ExtensionRecord} record */
  async put(record) {
    const fylo = await this.fyloHelper.database(); return await fylo.putData(Fylo.Collections.EXTENSIONS, record) }

  /** @returns {Promise<Array<ExtensionRecord & {docId: string}>>} */
  async listForOwner(owner) {
    const fylo = await this.fyloHelper.database()
    const documents = await Fylo.collect(fylo.findDocs(Fylo.Collections.EXTENSIONS, {
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

  async update(docId, patch) {
    const fylo = await this.fyloHelper.database(); await fylo.patchDoc(Fylo.Collections.EXTENSIONS, { [docId]: patch }) }
  async delete(docId) {
    const fylo = await this.fyloHelper.database(); await fylo.delDoc(Fylo.Collections.EXTENSIONS, docId) }
}

