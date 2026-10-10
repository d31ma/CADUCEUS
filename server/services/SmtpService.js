import { TrackingTokens } from '../shared/delivery/TrackingTokens.js'
import { ServerEnvironment } from '../shared/runtime/ServerEnvironment.js'
import { SmtpRules } from '../shared/delivery/SmtpRules.js'
import { ConsoleSmtpAdapter } from '../shared/delivery/ConsoleSmtpAdapter.js'
import { SmtpDirectAdapter } from '../shared/delivery/SmtpDirectAdapter.js'
import { SmtpDeliveryError } from '../shared/errors/SmtpDeliveryError.js'

/** @import { SendRequest } from '../models/SendRequest.js' */
/** @import { SmtpAdapter } from '../models/SmtpAdapter.js' */

/** Outbound mail delivery for the self-hosted server. */
@Service
export default class SmtpService {
  _adapter = null

  /** @returns {SmtpAdapter} */
  getAdapter() {
    if (!this._adapter) this._adapter = SmtpRules.createSmtpAdapter()
    return this._adapter
  }

  /** @param {SendRequest} msg */
  async send(msg) {
    return await this.getAdapter().send(msg)
  }

  /** @param {string} from @param {SendRequest} msg */
  async sendEmail(from, msg) {
    return await this.getAdapter().sendEmail(from, msg)
  }
}

