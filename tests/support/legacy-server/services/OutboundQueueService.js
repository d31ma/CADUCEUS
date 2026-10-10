import { ServerEnvironment } from '../shared/runtime/ServerEnvironment.js'
import { OutboundQueueRules } from '../shared/delivery/OutboundQueueRules.js'

@Service
export default class OutboundQueueService {
  /** @type {OutboxHelper} */
  @Needs
  outboxHelper

  /** @type {SmtpService} */
  @Needs
  smtpService

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {ScheduledHelper} */
  @Needs
  scheduledHelper

  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /** @type {SuppressedHelper} */
  @Needs
  suppressedHelper

  maxAttempts = OutboundQueueRules.MAX_ATTEMPTS

  _processing = null

  /**
   * Queue a message for delivery.
   *
   * @param {string} sender
   * @param {import('./SmtpService.js').SendRequest} email
   * @param {{ owner?: string, availableAt?: number, trackingId?: string, id?: string, scheduledId?: string, attachmentEmailId?: string }} [opts]
   * @returns {Promise<string>} The outbox entry id.
   */
  async enqueue(sender, email, opts = {}) {
    const availableAt = opts.availableAt ?? Date.now()
    const id = opts.id || `out-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    await this.outboxHelper.put({
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
    const entries = Object.values(await this.outboxHelper.listAll())
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
    const due = await this.outboxHelper.findDue(now)
    let delivered = 0, retried = 0, bounced = 0

    for (const [docId, entry] of Object.entries(due)) {
      if (entry.status === 'sending' && (entry.claimedAt ?? 0) > now - OutboundQueueRules.DELIVERY_LEASE_MS) continue
      await this.outboxHelper.update(docId, { status: 'sending', claimedAt: now })

      try {
        await this.smtpService.sendEmail(entry.sender, await this._hydrateAttachments(entry))
        await this.outboxHelper.delete(docId)
        await this._finishScheduled(entry, 'delivered')
        delivered++
      } catch (err) {
        const attempts = (entry.attempts ?? 0) + 1
        const plan = OutboundQueueRules.deliveryFailurePlan(entry.email, err, attempts >= this.maxAttempts)
        if (plan.permanent.length) {
          const failedEntry = { ...entry, email: OutboundQueueRules.deliveryForRecipients(entry.email, plan.permanent) }
          await this._bounce(failedEntry, err, attempts)
          await this._suppressPermanent(plan.permanent)
          bounced++
        }
        if (plan.transient.length) {
          await this.outboxHelper.update(docId, OutboundQueueRules.retryPatch(entry.email, plan.transient, attempts, now, this._backoff(attempts), this._message(err)))
          retried++
        } else {
          await this.outboxHelper.delete(docId)
          await this._finishScheduled(entry, 'failed')
        }
      }
    }
    return { delivered, retried, bounced }
  }

  /** FYLO bucket objects remain normalized; queue rows store only their owner id. */
  async _hydrateAttachments(entry) {
    if (!entry.attachmentEmailId || !this.attachmentsHelper) return entry.email
    const records = await this.attachmentsHelper.list(entry.attachmentEmailId)
    if (!records.length) return entry.email
    const attachments = []
    for (const record of records) {
      attachments.push({
        filename: record.filename,
        contentType: record.contentType,
        contentBase64: Buffer.from(await this.attachmentsHelper.readContent(record)).toString('base64'),
        disposition: record.disposition,
        ...(record.contentId ? { contentId: record.contentId } : {}),
      })
    }
    return { ...entry.email, attachments }
  }

  /** Remove the management record once delivery reaches a terminal state. */
  async _finishScheduled(entry, status) {
    if (!entry.scheduledId || !this.scheduledHelper) return
    try {
      const [docId] = await this.scheduledHelper.findById(entry.scheduledId)
      if (docId) {
        await this.scheduledHelper.update(docId, { status, updatedAt: Date.now() })
        await this.scheduledHelper.delete(docId)
      }
    } catch (error) {
      // Delivery is already terminal. A stale management row is preferable to
      // re-queuing and sending a duplicate message.
      console.error('[send] could not clean up scheduled metadata:', error)
    }
  }

  /** Upgrade legacy metadata-only schedules into durable queue work. */
  async _reconcileScheduled() {
    if (!this.scheduledHelper) return
    const records = await this.scheduledHelper.listAll()
    for (const [scheduledDocId, entry] of Object.entries(records)) {
      if (entry.transport === 'external') continue
      if (entry.status === 'delivered' || entry.status === 'failed') continue
      const [queueDocId] = await this.outboxHelper.findById(entry.queueId ?? entry.id)
      if (queueDocId) continue
      const queueId = await this.enqueue(entry.sender, entry.email, {
        id: entry.id,
        owner: entry.owner ?? entry.sender,
        availableAt: entry.sendAt,
        trackingId: entry.email.trackingId,
        scheduledId: entry.id,
        attachmentEmailId: entry.id,
      })
      await this.scheduledHelper.update(scheduledDocId, { queueId, status: 'scheduled', updatedAt: Date.now() })
    }
  }

  /** @param {number} attempts @returns {number} */
  _backoff(attempts) {
    return OutboundQueueRules.BACKOFF_MS[Math.min(attempts, OutboundQueueRules.BACKOFF_MS.length) - 1]
  }

  /** @param {unknown} err @returns {string} */
  _message(err) {
    return String(err instanceof Error ? err.message : err ?? 'unknown error').replace(/[\r\n\0]+/g, ' ').slice(0, 500)
  }

  /** Suppress only recipients that returned a permanent SMTP failure. */
  async _suppressPermanent(recipients) {
    if (!this.suppressedHelper) return
    for (const recipient of recipients) await this.suppressedHelper.suppress(recipient, 'bounce')
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
    const recipients = OutboundQueueRules.deliveryRecipients(email).join(', ')
    const daemon = `mailer-daemon@${ServerEnvironment.value('MAIL_HOSTNAME') || domain || 'localhost'}`
    const inboxFolder = await this.categoryMappingService?.pathFor(sender, domain, 'inbox') || 'inbox'

    await this.emailsHelper.put({
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

