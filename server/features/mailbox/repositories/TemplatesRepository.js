import { Collections, collect } from '../../../shared/persistence/Fylo.js'

export class TemplatesRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {TemplateEntry} entry
   * @returns {Promise<string>}
   */
  async put(entry) {
    return await this.fylo.putData(Collections.TEMPLATES, entry)
  }

  /**
   * @param {string} owner
   * @returns {Promise<Record<string, TemplateEntry>>}
   */
  async listForOwner(owner) {
    return /** @type {Promise<Record<string, TemplateEntry>>} */ (
      collect(this.fylo.findDocs(Collections.TEMPLATES, { $ops: [{ owner: { $eq: owner } }] }).collect())
    )
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, TemplateEntry | null]>}
   */
  async findById(id) {
    const docs = await collect(this.fylo.findDocs(Collections.TEMPLATES, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {TemplateEntry} */ (entry[1])] : [null, null]
  }

  /**
   * @param {string} docId
   * @param {Partial<TemplateEntry>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    await this.fylo.patchDoc(Collections.TEMPLATES, { [docId]: patch })
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    await this.fylo.delDoc(Collections.TEMPLATES, docId)
  }
}
