import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class TemplatesHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @param {TemplateEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.TEMPLATES, entry)
  }

  /**
   * @param {string} owner
   * @returns {Promise<Record<string, TemplateEntry>>}
   */
  async listForOwner(owner) {
    const fylo = await this.fyloHelper.database()
    return /** @type {Promise<Record<string, TemplateEntry>>} */ (
      Fylo.collect(fylo.findDocs(Fylo.Collections.TEMPLATES, { $ops: [{ owner: { $eq: owner } }] }).collect())
    )
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, TemplateEntry | null]>}
   */
  async findById(id) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.TEMPLATES, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {TemplateEntry} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} docId
   * @param {Partial<TemplateEntry>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.TEMPLATES, { [docId]: patch })
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    const fylo = await this.fyloHelper.database()
    await fylo.delDoc(Fylo.Collections.TEMPLATES, docId)
  }
}
