import { Buffer } from 'node:buffer'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

export class OutboundQueueRules {
  /** @import { OutboxEntry } from '../repositories/OutboxRepository.js' */

  /**
   * Outbound delivery queue with retry and bounce handling.
   *
   * A real MTA cannot deliver synchronously and give up on the first hiccup:
   * remote servers greylist, time out, or are briefly unreachable. This service
   * persists outbound messages in the `outbox`, retries transient failures with
   * exponential backoff, and — on a permanent failure or once retries are
   * exhausted — "bounces" by dropping a delivery-failure notice into the sender's
   * mailbox (a local DSN). It uses the direct-to-MX SMTP delivery service.
   *
   */

  /** Backoff before attempt N (1-indexed), in ms: ~1m, 5m, 15m, 1h, 4h. */
  static BACKOFF_MS = [60_000, 300_000, 900_000, 3_600_000, 14_400_000]

  static MAX_ATTEMPTS = OutboundQueueRules.BACKOFF_MS.length

  static DELIVERY_LEASE_MS = 5 * 60_000

  /** @param {import('./SmtpService.js').SendRequest} email */
  static deliveryRecipients(email) {
    const requested = email.envelopeRecipients || [...(email.to || []), ...(email.cc || []), ...(email.bcc || [])]
    return [...new Map(requested.map(address => [String(address).toLowerCase(), String(address)])).values()]
  }

  /** @param {import('./SmtpService.js').SendRequest} email @param {string[]} recipients */
  static deliveryForRecipients(email, recipients) {
    return { ...email, envelopeRecipients: [...recipients] }
  }

  /** @param {import('./SmtpService.js').SendRequest} email @param {unknown} error @param {boolean} exhausted */
  static deliveryFailurePlan(email, error, exhausted = false) {
    const requested = OutboundQueueRules.deliveryRecipients(email)
    if (exhausted) return { permanent: requested, transient: [] }
    const results = Array.isArray(/** @type {any} */ (error)?.deliveryResults) ? /** @type {any} */ (error).deliveryResults : []
    if (!results.length) return OutboundQueueRules.isPermanentReply(/** @type {any} */ (error)?.responseCode)
      ? { permanent: requested, transient: [] }
      : { permanent: [], transient: requested }

    const delivered = OutboundQueueRules.resultRecipients(results, 'deliveredRecipients')
    const failed = OutboundQueueRules.resultRecipients(results, 'failedRecipients')
    const permanent = OutboundQueueRules.permanentRecipients(results)
    return {
      permanent: requested.filter(value => permanent.has(value.toLowerCase())),
      // A recipient omitted by an adapter result is unconfirmed, never delivered.
      // Retain it for retry so malformed diagnostics cannot silently lose mail.
      transient: requested.filter(value => {
        const key = value.toLowerCase()
        return !permanent.has(key) && (failed.has(key) || !delivered.has(key))
      }),
    }
  }

  /** @param {Array<Record<string, any>>} results @param {'deliveredRecipients'|'failedRecipients'} field */
  static resultRecipients(results, field) {
    return new Set(results.flatMap(result => result[field] || []).map(value => String(value).toLowerCase()))
  }

  /** @param {Array<Record<string, any>>} results */
  static permanentRecipients(results) {
    return new Set(results.flatMap(result => {
      if (Array.isArray(result.recipientFailures) && result.recipientFailures.length) {
        return result.recipientFailures.filter(failure => OutboundQueueRules.isPermanentReply(failure.responseCode)).map(failure => failure.recipient)
      }
      return OutboundQueueRules.isPermanentReply(result.responseCode) ? result.failedRecipients || [] : []
    }).map(value => String(value).toLowerCase()))
  }

  /** @param {unknown} value */
  static isPermanentReply(value) { const code = Number(value); return code >= 500 && code < 600 }

  /** @param {import('./SmtpService.js').SendRequest} email @param {string[]} recipients @param {number} attempts @param {number} now @param {number} backoff @param {string} message @returns {Partial<OutboxEntry>} */
  static retryPatch(email, recipients, attempts, now, backoff, message) {
    return { status: 'queued', attempts, nextAttempt: now + backoff, lastError: message, claimedAt: 0, email: OutboundQueueRules.deliveryForRecipients(email, recipients) }
  }
}
