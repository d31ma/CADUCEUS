import { Fylo } from '../shared/persistence/Fylo.js'
import { DomainsRecords } from '../shared/domains/DomainsRecords.js'

/** @import { DomainConfig } from '../types' */
/** @import { RouteRule } from '../types' */

@Helper
export default class DomainsHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @param {string[]} allowedDomains
   * @returns {Promise<DomainConfig[]>}
   */
  async list(allowedDomains) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.DOMAINS, { $ops: [] }).collect())
    return Object.values(docs)
      .filter(d => allowedDomains.includes(d.domain))
      .map(d => DomainsRecords.deserialize(/** @type {RawDomainDoc} */ (d)))
  }

  /**
   * @param {string} domain
   * @returns {Promise<[string | null, DomainConfig | null]>}
   */
  async findEntry(domain) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.DOMAINS, { $ops: [{ domain: { $eq: domain } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], DomainsRecords.deserialize(/** @type {RawDomainDoc} */ (entry[1]))] : [null, null]
  }

  /**
   * @param {DomainConfig} config
   * @returns {Promise<string>}
   */
  async put(config) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.DOMAINS, DomainsRecords.serialize(config))
  }

  /**
   * @param {string} docId
   * @param {RouteRule[]} routes
   * @returns {Promise<void>}
   */
  async updateRoutes(docId, routes) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.DOMAINS, { [docId]: { routes: JSON.stringify(routes) } })
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    const fylo = await this.fyloHelper.database()
    await fylo.delDoc(Fylo.Collections.DOMAINS, docId)
  }
}

