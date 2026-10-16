/** @import { RawDomainDoc, DomainConfig } from '../../types' */
import { Fylo } from '../persistence/Fylo.js'

export class DomainsRecords {
  /** @param {RawDomainDoc} raw @returns {DomainConfig} */
  static deserialize(raw) {
    return { ...raw, routes: typeof raw.routes === 'string' ? JSON.parse(raw.routes) : (raw.routes ?? []) }
  }

  /** @param {DomainConfig} config @returns {RawDomainDoc} */
  static serialize(config) {
    return { ...config, routes: JSON.stringify(config.routes) }
  }
}
