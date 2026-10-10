import { Fylo } from '../../../shared/persistence/Fylo.js'
/** @import { DomainConfig } from '../../../types' */
/** @import { RouteRule } from '../../../types' */

export class DomainsRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {string[]} allowedDomains
   * @returns {Promise<DomainConfig[]>}
   */
  async list(allowedDomains) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.DOMAINS, { $ops: [] }).collect())
    return Object.values(docs)
      .filter(d => allowedDomains.includes(d.domain))
      .map(d => deserialize(/** @type {RawDomainDoc} */ (d)))
  }

  /**
   * @param {string} domain
   * @returns {Promise<[string | null, DomainConfig | null]>}
   */
  async findEntry(domain) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.DOMAINS, { $ops: [{ domain: { $eq: domain } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], deserialize(/** @type {RawDomainDoc} */ (entry[1]))] : [null, null]
  }

  /**
   * @param {DomainConfig} config
   * @returns {Promise<string>}
   */
  async put(config) {
    return await this.fylo.putData(Fylo.Collections.DOMAINS, serialize(config))
  }

  /**
   * @param {string} docId
   * @param {RouteRule[]} routes
   * @returns {Promise<void>}
   */
  async updateRoutes(docId, routes) {
    await this.fylo.patchDoc(Fylo.Collections.DOMAINS, { [docId]: { routes: JSON.stringify(routes) } })
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    await this.fylo.delDoc(Fylo.Collections.DOMAINS, docId)
  }
}

/** @param {RawDomainDoc} raw @returns {DomainConfig} */
function deserialize(raw) {
  return { ...raw, routes: typeof raw.routes === 'string' ? JSON.parse(raw.routes) : (raw.routes ?? []) }
}

/** @param {DomainConfig} config @returns {RawDomainDoc} */
function serialize(config) {
  return { ...config, routes: JSON.stringify(config.routes) }
}
