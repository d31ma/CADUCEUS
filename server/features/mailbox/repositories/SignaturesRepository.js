import { Fylo } from '../../../shared/persistence/Fylo.js'

export class SignaturesRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {SignatureEntry} entry */
  async put(entry) { return await this.fylo.putData(Fylo.Collections.SIGNATURES, entry) }

  /** @param {string} owner @returns {Promise<Record<string, SignatureEntry>>} */
  async listForOwner(owner) {
    return /** @type {Promise<Record<string, SignatureEntry>>} */ (
      Fylo.collect(this.fylo.findDocs(Fylo.Collections.SIGNATURES, { $ops: [{ owner: { $eq: owner } }] }).collect())
    )
  }

  /** @param {string} id @returns {Promise<[string | null, SignatureEntry | null]>} */
  async findById(id) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.SIGNATURES, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {SignatureEntry} */ (entry[1])] : [null, null]
  }

  /** @param {string} docId @param {Partial<SignatureEntry>} patch */
  async update(docId, patch) { await this.fylo.patchDoc(Fylo.Collections.SIGNATURES, { [docId]: patch }) }

  /** @param {string} docId */
  async delete(docId) { await this.fylo.delDoc(Fylo.Collections.SIGNATURES, docId) }

  /**
   * An address has at most one default. Remove newly claimed addresses from
   * every other signature in one FYLO patch operation.
   *
   * @param {string} owner
   * @param {string[]} addresses
   * @param {string} excludeId
   */
  async releaseDefaults(owner, addresses, excludeId) {
    if (addresses.length === 0) return
    const claimed = new Set(addresses)
    const records = await this.listForOwner(owner)
    const patch = {}
    for (const [docId, entry] of Object.entries(records)) {
      if (entry.id === excludeId) continue
      const current = entry.defaultFor ?? []
      const next = current.filter(address => !claimed.has(address))
      if (next.length !== current.length) patch[docId] = { defaultFor: next, updatedAt: Date.now() }
    }
    if (Object.keys(patch).length) await this.fylo.patchDoc(Fylo.Collections.SIGNATURES, patch)
  }
}
