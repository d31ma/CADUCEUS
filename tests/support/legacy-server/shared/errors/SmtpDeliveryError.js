import { SmtpRules } from '../delivery/SmtpRules.js'

export class SmtpDeliveryError extends Error {
  /** @param {Array<Record<string,any>>} results */
  constructor(results) {
    const failed = results.filter(result => result.failedRecipients?.length)
    super(`Direct SMTP delivery failed — ${failed.map(result => `${result.domain}: ${result.error || 'recipient rejected'}`).join('; ')}`)
    this.name = 'SmtpDeliveryError'
    this.deliveryResults = results
    this.deliveredRecipients = SmtpRules.uniqueRecipients(results.flatMap(result => result.deliveredRecipients || []))
    this.failedRecipients = SmtpRules.uniqueRecipients(failed.flatMap(result => result.failedRecipients || []))
    const failures = failed.flatMap(SmtpRules.recipientFailures)
    this.responseCode = failures.every(failure => SmtpRules.permanentReply(failure.responseCode)) ? 550 : 451
  }
}
