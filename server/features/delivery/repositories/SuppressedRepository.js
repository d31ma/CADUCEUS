import { Fylo } from '../../../shared/persistence/Fylo.js'
/** @import { SuppressedAddress } from '../../../types' */

export class SuppressedRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @returns {Promise<SuppressedAddress[]>}
   */
  async list() {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [] }).collect())
    return /** @type {SuppressedAddress[]} */ (Object.values(docs))
  }

  /**
   * @returns {Promise<Set<string>>}
   */
  async asSet() {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [] }).collect())
    return new Set(Object.values(docs).map(r => r.address))
  }

  /**
   * @param {string} address
   * @param {string} reason
   * @returns {Promise<void>}
   */
  async suppress(address, reason) {
    const existing = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [{ address: { $eq: address } }] }).collect())
    if (Object.keys(existing).length > 0) return
    await this.fylo.putData(Fylo.Collections.SUPPRESSED, { address, reason, suppressedAt: new Date().toISOString() })
  }

  /**
   * @param {string} address
   * @returns {Promise<void>}
   */
  async delete(address) {
    const existing = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.SUPPRESSED, { $ops: [{ address: { $eq: address } }] }).collect())
    await Promise.all(Object.keys(existing).map(docId => this.fylo.delDoc(Fylo.Collections.SUPPRESSED, docId)))
  }
}
