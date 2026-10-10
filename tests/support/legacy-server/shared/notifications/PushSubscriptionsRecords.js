import { createHash } from 'node:crypto'
import { Fylo } from '../persistence/Fylo.js'

export class PushSubscriptionsRecords {
  /** @param {string} endpoint @returns {string} */
  static subscriptionId(endpoint) {
    return createHash('sha256').update(endpoint).digest('hex')
  }
}
