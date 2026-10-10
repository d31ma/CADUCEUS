import { Buffer } from 'node:buffer'

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
 * @typedef {import('../repositories/outbox.js').OutboxEntry} OutboxEntry
 */

/** Backoff before attempt N (1-indexed), in ms: ~1m, 5m, 15m, 1h, 4h. */
const BACKOFF_MS = [60_000, 300_000, 900_000, 3_600_000, 14_400_000]
const MAX_ATTEMPTS = BACKOFF_MS.length
export const DELIVERY_LEASE_MS = 5 * 60_000

export class OutboundQueueService {
  /**
   * @param {import('../repositories/outbox.js').OutboxRepository} outbox
   * @param {import('./smtp.js').SmtpService} smtp
   * @param {import('../../mailbox/repositories/emails.js').EmailsRepository} emails
   * @param {import('../repositories/scheduled.js').ScheduledRepository} [scheduled]
   * @param {import('../../mailbox/repositories/attachments.js').AttachmentsRepository} [attachments]
   * @param {import('../../mailbox/services/category-mappings.js').CategoryMappingService} [categoryMappings]
   * @param {import('../repositories/suppressed.js').SuppressedRepository} [suppressed]
   */
  constructor(outbox, smtp, emails, scheduled, attachments, categoryMappings, suppressed) {
    this.outbox = outbox
    this.smtp = smtp
    this.emails = emails
    this.scheduled = scheduled
    this.attachments = attachments
    this.categoryMappings = categoryMappings
    this.suppressed = suppressed
    this.maxAttempts = MAX_ATTEMPTS
    this._processing = null
  }

  /**
   * Queue a message for delivery.
   *
   * @param {string} sender
   * @param {import('./smtp.js').SendRequest} email
   * @param {{ owner?: string, availableAt?: number, trackingId?: string, id?: string, scheduledId?: string, attachmentEmailId?: string }} [opts]
   * @returns {Promise<string>} The outbox entry id.
   */
  async enqueue(sender, email, opts = {}) {
    const availableAt = opts.availableAt ?? Date.now()
    const id = opts.id || `out-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    await this.outbox.put({
      id,
      sender,
      owner: opts.owner ?? sender,
      email,
      attempts: 0,
      status: 'queued',
      nextAttempt: availableAt,
      expiresAt: availableAt, // keep in sync so legacy findExpired() still sees it
      createdAt: Date.now(),
      ...(opts.trackingId ? { trackingId: opts.trackingId } : {}),
      ...(opts.scheduledId ? { scheduledId: opts.scheduledId } : {}),
      ...(opts.attachmentEmailId ? { attachmentEmailId: opts.attachmentEmailId } : {}),
    })
    return id
  }

  /**
   * Attempt delivery of every due entry once. Successes are removed; transient
   * failures are rescheduled with backoff; permanent failures (or exhausted
   * retries) are bounced. Intended to be called on a short interval.
   *
   * @param {number} [now]
   * @returns {Promise<{ delivered: number, retried: number, bounced: number }>}
   */
  async processDue(now = Date.now()) {
    // The application owns one persistent FYLO writer. Serializing overlapping
    // timer/manual ticks inside that writer closes the read-then-claim race.
    if (this._processing) return await this._processing
    this._processing = this._processDue(now)
    try { return await this._processing } finally { this._processing = null }
  }

  /** Secret-free queue health for worker logs and signed operations. */
  async status(now = Date.now()) {
    const entries = Object.values(await this.outbox.listAll())
    const created = entries.map(entry => Number(entry.createdAt) || now)
    return {
      depth: entries.length,
      queued: entries.filter(entry => entry.status !== 'sending').length,
      sending: entries.filter(entry => entry.status === 'sending').length,
      due: entries.filter(entry => (entry.nextAttempt ?? entry.expiresAt) <= now).length,
      oldestAgeMs: entries.length ? Math.max(0, now - Math.min(...created)) : 0,
      attempts: entries.reduce((total, entry) => total + (Number(entry.attempts) || 0), 0),
    }
  }

  /** @param {number} now */
  async _processDue(now) {
    await this._reconcileScheduled()
    const due = await this.outbox.findDue(now)
    let delivered = 0, retried = 0, bounced = 0

    for (const [docId, entry] of Object.entries(due)) {
      if (entry.status === 'sending' && (entry.claimedAt ?? 0) > now - DELIVERY_LEASE_MS) continue
      await this.outbox.update(docId, { status: 'sending', claimedAt: now })

      try {
        await this.smtp.sendEmail(entry.sender, await this._hydrateAttachments(entry))
        await this.outbox.delete(docId)
        await this._finishScheduled(entry, 'delivered')
        delivered++
      } catch (err) {
        const attempts = (entry.attempts ?? 0) + 1
        const plan = deliveryFailurePlan(entry.email, err, attempts >= this.maxAttempts)
        if (plan.permanent.length) {
          const failedEntry = { ...entry, email: deliveryForRecipients(entry.email, plan.permanent) }
          await this._bounce(failedEntry, err, attempts)
          await this._suppressPermanent(plan.permanent)
          bounced++
        }
        if (plan.transient.length) {
          await this.outbox.update(docId, retryPatch(entry.email, plan.transient, attempts, now, this._backoff(attempts), this._message(err)))
          retried++
        } else {
          await this.outbox.delete(docId)
          await this._finishScheduled(entry, 'failed')
        }
      }
    }
    return { delivered, retried, bounced }
  }

  /** FYLO bucket objects remain normalized; queue rows store only their owner id. */
  async _hydrateAttachments(entry) {
    if (!entry.attachmentEmailId || !this.attachments) return entry.email
    const records = await this.attachments.list(entry.attachmentEmailId)
    if (!records.length) return entry.email
    const attachments = []
    for (const record of records) {
      attachments.push({
        filename: record.filename,
        contentType: record.contentType,
        contentBase64: Buffer.from(await this.attachments.readContent(record)).toString('base64'),
        disposition: record.disposition,
        ...(record.contentId ? { contentId: record.contentId } : {}),
      })
    }
    return { ...entry.email, attachments }
  }

  /** Remove the management record once delivery reaches a terminal state. */
  async _finishScheduled(entry, status) {
    if (!entry.scheduledId || !this.scheduled) return
    try {
      const [docId] = await this.scheduled.findById(entry.scheduledId)
      if (docId) {
        await this.scheduled.update(docId, { status, updatedAt: Date.now() })
        await this.scheduled.delete(docId)
      }
    } catch (error) {
      // Delivery is already terminal. A stale management row is preferable to
      // re-queuing and sending a duplicate message.
      console.error('[send] could not clean up scheduled metadata:', error)
    }
  }

  /** Upgrade legacy metadata-only schedules into durable queue work. */
  async _reconcileScheduled() {
    if (!this.scheduled) return
    const records = await this.scheduled.listAll()
    for (const [scheduledDocId, entry] of Object.entries(records)) {
      if (entry.transport === 'external') continue
      if (entry.status === 'delivered' || entry.status === 'failed') continue
      const [queueDocId] = await this.outbox.findById(entry.queueId ?? entry.id)
      if (queueDocId) continue
      const queueId = await this.enqueue(entry.sender, entry.email, {
        id: entry.id,
        owner: entry.owner ?? entry.sender,
        availableAt: entry.sendAt,
        trackingId: entry.email.trackingId,
        scheduledId: entry.id,
        attachmentEmailId: entry.id,
      })
      await this.scheduled.update(scheduledDocId, { queueId, status: 'scheduled', updatedAt: Date.now() })
    }
  }

  /** @param {number} attempts @returns {number} */
  _backoff(attempts) {
    return BACKOFF_MS[Math.min(attempts, BACKOFF_MS.length) - 1]
  }

  /** @param {unknown} err @returns {string} */
  _message(err) {
    return String(err instanceof Error ? err.message : err ?? 'unknown error').replace(/[\r\n\0]+/g, ' ').slice(0, 500)
  }

  /** Suppress only recipients that returned a permanent SMTP failure. */
  async _suppressPermanent(recipients) {
    if (!this.suppressed) return
    for (const recipient of recipients) await this.suppressed.suppress(recipient, 'bounce')
  }

  /**
   * Deliver a delivery-failure notice to the sender's own mailbox.
   *
   * @param {OutboxEntry} entry
   * @param {unknown} err
   * @param {number} attempts
   * @returns {Promise<void>}
   */
  async _bounce(entry, err, attempts) {
    const sender = entry.owner ?? entry.sender
    const domain = sender.split('@')[1] || ''
    const email = entry.email
    const recipients = deliveryRecipients(email).join(', ')
    const daemon = `mailer-daemon@${process.env.MAIL_HOSTNAME || domain || 'localhost'}`
    const inboxFolder = await this.categoryMappings?.pathFor(sender, domain, 'inbox') || 'inbox'

    await this.emails.put({
      id: `bounce-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
      domain,
      recipient: sender,
      sender: daemon,
      subject: `Undelivered: ${email.subject}`,
      body:
        `Your message could not be delivered to ${recipients} after ${attempts} attempt(s).\n\n` +
        `Reason: ${this._message(err)}\n\n` +
        `----- Original message -----\n` +
        `To: ${recipients}\nSubject: ${email.subject}\n\n${email.text ?? ''}`,
      folder: inboxFolder,
      read: false,
      starred: false,
      receivedAt: new Date().toISOString(),
      processed: true,
      bounce: true,
    })
  }
}

/** @param {import('./smtp.js').SendRequest} email */
export function deliveryRecipients(email) {
  const requested = email.envelopeRecipients || [...(email.to || []), ...(email.cc || []), ...(email.bcc || [])]
  return [...new Map(requested.map(address => [String(address).toLowerCase(), String(address)])).values()]
}

/** @param {import('./smtp.js').SendRequest} email @param {string[]} recipients */
export function deliveryForRecipients(email, recipients) {
  return { ...email, envelopeRecipients: [...recipients] }
}

/** @param {import('./smtp.js').SendRequest} email @param {unknown} error @param {boolean} exhausted */
export function deliveryFailurePlan(email, error, exhausted = false) {
  const requested = deliveryRecipients(email)
  if (exhausted) return { permanent: requested, transient: [] }
  const results = Array.isArray(/** @type {any} */ (error)?.deliveryResults) ? /** @type {any} */ (error).deliveryResults : []
  if (!results.length) return isPermanentReply(/** @type {any} */ (error)?.responseCode)
    ? { permanent: requested, transient: [] }
    : { permanent: [], transient: requested }

  const delivered = resultRecipients(results, 'deliveredRecipients')
  const failed = resultRecipients(results, 'failedRecipients')
  const permanent = permanentRecipients(results)
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
function resultRecipients(results, field) {
  return new Set(results.flatMap(result => result[field] || []).map(value => String(value).toLowerCase()))
}

/** @param {Array<Record<string, any>>} results */
function permanentRecipients(results) {
  return new Set(results.flatMap(result => {
    if (Array.isArray(result.recipientFailures) && result.recipientFailures.length) {
      return result.recipientFailures.filter(failure => isPermanentReply(failure.responseCode)).map(failure => failure.recipient)
    }
    return isPermanentReply(result.responseCode) ? result.failedRecipients || [] : []
  }).map(value => String(value).toLowerCase()))
}

/** @param {unknown} value */
function isPermanentReply(value) { const code = Number(value); return code >= 500 && code < 600 }

/** @param {import('./smtp.js').SendRequest} email @param {string[]} recipients @param {number} attempts @param {number} now @param {number} backoff @param {string} message @returns {Partial<OutboxEntry>} */
function retryPatch(email, recipients, attempts, now, backoff, message) {
  return { status: 'queued', attempts, nextAttempt: now + backoff, lastError: message, claimedAt: 0, email: deliveryForRecipients(email, recipients) }
}
