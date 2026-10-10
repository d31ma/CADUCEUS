import { Collections, collect } from '../../../platform/persistence/Fylo.js'
/** @import { SuppressedAddress } from '../../../types' */

export class SuppressedRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @returns {Promise<SuppressedAddress[]>}
   */
  async list() {
    const docs = await collect(this.fylo.findDocs(Collections.SUPPRESSED, { $ops: [] }).collect())
    return /** @type {SuppressedAddress[]} */ (Object.values(docs))
  }

  /**
   * @returns {Promise<Set<string>>}
   */
  async asSet() {
    const docs = await collect(this.fylo.findDocs(Collections.SUPPRESSED, { $ops: [] }).collect())
    return new Set(Object.values(docs).map(r => r.address))
  }

  /**
   * @param {string} address
   * @param {string} reason
   * @returns {Promise<void>}
   */
  async suppress(address, reason) {
    const existing = await collect(this.fylo.findDocs(Collections.SUPPRESSED, { $ops: [{ address: { $eq: address } }] }).collect())
    if (Object.keys(existing).length > 0) return
    await this.fylo.putData(Collections.SUPPRESSED, { address, reason, suppressedAt: new Date().toISOString() })
  }

  /**
   * @param {string} address
   * @returns {Promise<void>}
   */
  async delete(address) {
    const existing = await collect(this.fylo.findDocs(Collections.SUPPRESSED, { $ops: [{ address: { $eq: address } }] }).collect())
    await Promise.all(Object.keys(existing).map(docId => this.fylo.delDoc(Collections.SUPPRESSED, docId)))
  }
}
