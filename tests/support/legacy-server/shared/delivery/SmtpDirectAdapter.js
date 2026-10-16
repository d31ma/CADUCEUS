/** @import { SendRequest } from '../../models/SendRequest.js' */
import { readFileSync } from 'node:fs'
import { TrackingTokens } from './TrackingTokens.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'
import { SmtpRules } from './SmtpRules.js'
import { SmtpDeliveryError } from '../errors/SmtpDeliveryError.js'

/** Direct-to-MX delivery without an email relay or cloud mail provider. */
export class SmtpDirectAdapter {
  /**
   * @param {object} [deps] Transport seams. Delivery opens real SMTP
   *   connections to remote MX hosts, so tests supply their own.
   * @param {(options: object) => { sendMail: (message: object) => Promise<any> }} [deps.createTransport]
   * @param {(domain: string) => Promise<string[]>} [deps.resolveMx]
   */
  constructor(deps = {}) {
    this.deps = deps
  }

  /** @param {string} from @param {SendRequest} msg */
  async sendEmail(from, msg) {
    return this.send({ ...msg, replyTo: msg.replyTo ?? [from] }, from)
  }

  /** @param {SendRequest} msg @param {string} [fromOverride] */
  async send(msg, fromOverride) {
    const hostname = ServerEnvironment.value('MAIL_HOSTNAME') || 'localhost'
    const from = fromOverride || ServerEnvironment.value('SMTP_FROM') || `postmaster@${hostname}`
    /** @type {(options: any) => { sendMail: (message: any) => Promise<any> }} */
    const createTransport = this.deps.createTransport
      ?? /** @type {any} */ ((await import('../../deps/npm/nodemailer.mjs')).default.createTransport)
    const resolveMx = this.deps.resolveMx ?? SmtpRules.resolveMailExchangers
    const dkim = SmtpRules.loadDkim()
    const recipients = SmtpRules.uniqueRecipients(msg.envelopeRecipients || [...(msg.to || []), ...(msg.cc || []), ...(msg.bcc || [])])
    const tracked = Boolean(msg.trackingId && (msg.openTracking || msg.linkTracking))
    const deliveries = SmtpRules.deliveryPlan(recipients, tracked)
    const results = []
    for (const delivery of deliveries) {
      results.push(await SmtpRules.deliverToDomain({ delivery, from, msg, hostname, dkim, createTransport, resolveMx }))
    }
    const failed = results.filter(result => result.failedRecipients.length)
    if (failed.length) throw new SmtpDeliveryError(results)
    const succeeded = results.filter(result => result.deliveredRecipients.length)
    return { messageId: String(succeeded[0].messageId || `direct-${Date.now()}`), results }
  }
}
