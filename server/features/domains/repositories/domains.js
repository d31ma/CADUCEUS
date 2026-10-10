import { Collections, collect } from '../../../platform/persistence/fylo.js'

/**
 * Fylo stores nested arrays as JSON strings. These helpers convert between
 * the typed DomainConfig interface and the flat shape stored on disk.
 * @typedef {Omit<import('../../../types').DomainConfig, 'routes'> & { routes: string | import('../../../types').RouteRule[] }} RawDomainDoc
 * @typedef {import('../../../types').DomainConfig} DomainConfig
 * @typedef {import('../../../types').RouteRule} RouteRule
 */

export class DomainsRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {string[]} allowedDomains
   * @returns {Promise<DomainConfig[]>}
   */
  async list(allowedDomains) {
    const docs = await collect(this.fylo.findDocs(Collections.DOMAINS, { $ops: [] }).collect())
    return Object.values(docs)
      .filter(d => allowedDomains.includes(d.domain))
      .map(d => deserialize(/** @type {RawDomainDoc} */ (d)))
  }

  /**
   * @param {string} domain
   * @returns {Promise<[string | null, DomainConfig | null]>}
   */
  async findEntry(domain) {
    const docs = await collect(this.fylo.findDocs(Collections.DOMAINS, { $ops: [{ domain: { $eq: domain } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], deserialize(/** @type {RawDomainDoc} */ (entry[1]))] : [null, null]
  }

  /**
   * @param {DomainConfig} config
   * @returns {Promise<string>}
   */
  async put(config) {
    return await this.fylo.putData(Collections.DOMAINS, serialize(config))
  }

  /**
   * @param {string} docId
   * @param {RouteRule[]} routes
   * @returns {Promise<void>}
   */
  async updateRoutes(docId, routes) {
    await this.fylo.patchDoc(Collections.DOMAINS, { [docId]: { routes: JSON.stringify(routes) } })
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    await this.fylo.delDoc(Collections.DOMAINS, docId)
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
