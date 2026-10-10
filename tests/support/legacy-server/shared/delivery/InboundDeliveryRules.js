import { randomBytes } from 'node:crypto'
import { FyloAccessContext } from '../persistence/FyloAccessContext.js'
import { PhishingDetection } from './PhishingDetection.js'
import { ListUnsubscribe } from './ListUnsubscribe.js'
import { Threading } from '../mailbox/Threading.js'
import { PersonalLevel } from '../mailbox/PersonalLevel.js'

export class InboundDeliveryRules {
  /** @param {string} address */
  static domainFromAddress(address) {
    return address.split('@')[1] || 'localhost'
  }
}
