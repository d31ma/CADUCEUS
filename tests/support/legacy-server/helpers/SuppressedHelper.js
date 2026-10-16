import { Fylo } from '../shared/persistence/Fylo.js'
/** @import { SuppressedAddress } from '../types' */

@Helper
export default class SuppressedHelper {
  /** @type {FyloHelper | undefined} */
  @Needs
  fyloHelper

  /**
   * @returns {Promise<SuppressedAddress[]>}
   */
  async list() {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [] }).collect())
    return /** @type {SuppressedAddress[]} */ (Object.values(docs))
  }

  /**
   * @returns {Promise<Set<string>>}
   */
  async asSet() {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [] }).collect())
    return new Set(Object.values(docs).map(r => r.address))
  }

  /**
   * @param {string} address
   * @param {string} reason
   * @returns {Promise<void>}
   */
  async suppress(address, reason) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const existing = await Fylo.collect(fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [{ address: { $eq: address } }] }).collect())
    if (Object.keys(existing).length > 0) return
    await fylo.putData(Fylo.Collections.SUPPRESSED, { address, reason, suppressedAt: new Date().toISOString() })
  }

  /**
   * @param {string} address
   * @returns {Promise<void>}
   */
  async delete(address) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const existing = await Fylo.collect(fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [{ address: { $eq: address } }] }).collect())
    await Promise.all(Object.keys(existing).map(docId => fylo.delDoc(Fylo.Collections.SUPPRESSED, docId)))
  }
}
