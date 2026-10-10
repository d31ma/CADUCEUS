import { MessageSource } from '../shared/mailbox/MessageSource.js'
import { Threading } from '../shared/mailbox/Threading.js'
import { TrackingTokens } from '../shared/delivery/TrackingTokens.js'
/** @import { SendRequest } from './SmtpService.js' */

@Service
export default class SendService {
  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {RawMessagesHelper} */
  @Needs
  rawMessagesHelper

  /** @type {ScheduledHelper} */
  @Needs
  scheduledHelper

  /** @type {OutboxHelper} */
  @Needs
  outboxHelper

  /** @type {OutboundQueueService} */
  @Needs
  outboundQueueService

  /** @type {SmtpService} */
  @Needs
  smtpService

  /** @type {ContactsService} */
  @Needs
  contactsService

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /**
   * Deliver now. A delivery hiccup doesn't fail the request — it queues for
   * retry (the worker retries with backoff and bounces if undeliverable). The
   * Sent copy is stored either way.
   *
   * @param {string} ownerEmail @param {string} senderEmail @param {SendRequest} req @param {string | undefined} trackingId
   * @returns {Promise<{ messageId: string, threadId: string, queued?: true, queueId?: string }>}
   */
  async sendNow(ownerEmail, senderEmail, req, trackingId) {
    const messageId = req.messageId || `sent-${Date.now().toString(36)}`
    const delivery = SendService.#deliveryRequest(req, trackingId)
    // Persist first so a fast recipient cannot present a valid tracking
    // capability before its exact-owner Sent row exists.
    const stored = await this.#storeSentEmail(trackingId, ownerEmail, senderEmail, messageId, req)
    let result
    try {
      result = await this.smtpService.sendEmail(senderEmail, delivery)
    } catch (err) {
      console.error('[send] immediate delivery failed, queuing for retry:', err)
			const failure = /** @type {{failedRecipients?:unknown}} */ (err)
			const failedRecipients = Array.isArray(failure?.failedRecipients) ? failure.failedRecipients.map(String) : []
			const retryDelivery = failedRecipients.length ? { ...delivery, envelopeRecipients: failedRecipients } : delivery
			const queueId = await this.outboundQueueService.enqueue(senderEmail, SendService.#queueRequest(retryDelivery), {
        owner: ownerEmail, availableAt: Date.now() + 60_000, trackingId, attachmentEmailId: messageId,
      })
      return { messageId, queueId, threadId: stored.threadId, queued: true }
    }
    // The locally issued ID is the canonical Sent-row identity. Transports may
    // normalize Message-ID formatting, but callers must always receive the ID
    // that can be opened through CADUCEUS.
    return { messageId, threadId: stored.threadId }
  }

  /**
   * Persist the canonical Sent projection before a Go-owned external-account
   * submission is made runnable. The route compensates this projection if the
   * durable provider journal cannot be created.
   */
  async recordExternalAccepted(ownerEmail, senderEmail, req, trackingId) {
    const messageId = req.messageId || `sent-${Date.now().toString(36)}`
    const stored = await this.#storeSentEmail(trackingId, ownerEmail, senderEmail, messageId, req)
    return { messageId, threadId: stored.threadId }
  }

  /** Remove a pre-submission Sent projection after enqueue compensation. */
  async rollbackExternalAccepted(messageId) {
    const [documentId] = await this.emailsHelper.findById(messageId)
    await Promise.all([
      this.attachmentsHelper.deleteForEmail(messageId),
      this.rawMessagesHelper.deleteForEmail(messageId),
    ])
    if (documentId) await this.emailsHelper.delete(documentId, messageId)
  }

  /**
   * Queue with an undo window. The entry id doubles as the undo token — the undo
   * route deletes it before `availableAt` to cancel the send.
   *
   * @param {string} ownerEmail @param {string} senderEmail @param {SendRequest} req @param {number} delayMs @param {string | undefined} trackingId
   * @returns {Promise<{ sent: true, undoId: string, threadId: string }>}
   */
  async delay(ownerEmail, senderEmail, req, delayMs, trackingId) {
    const undoId = `undo-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    const availableAt = Date.now() + delayMs
    const delivery = SendService.#deliveryRequest(req, trackingId)

    // Retry metadata so the delivery worker picks it up after the undo window
    // and on restart.
    await this.outboxHelper.put({
      id: undoId, email: SendService.#queueRequest(delivery), sender: senderEmail, owner: ownerEmail,
      expiresAt: availableAt, nextAttempt: availableAt, attempts: 0, status: 'queued', createdAt: Date.now(),
      attachmentEmailId: undoId,
      ...(trackingId ? { trackingId } : {}),
    })

    // Tracking pixel must work even before the delayed send fires.
    const stored = await this.#storeSentEmail(trackingId, ownerEmail, senderEmail, undoId, req)

    // Kick a delivery pass right after the undo window for low latency; the
    // recurring worker handles retries/bounces from there. processDue claims
    // each entry ('sending') so this and the worker won't double-send.
    setTimeout(() => {
      this.outboundQueueService.processDue().catch(e => console.error('[send] delayed processDue failed:', e))
    }, delayMs + 50)

    return { sent: true, undoId, threadId: stored.threadId }
  }

  /**
   * Schedule for a future absolute time.
   *
   * @param {string} ownerEmail @param {string} senderEmail @param {SendRequest} req @param {number} sendAtMs @param {string | undefined} trackingId
   * @returns {Promise<{ scheduled: true, id: string, threadId: string }>}
   */
  async schedule(ownerEmail, senderEmail, req, sendAtMs, trackingId) {
    const scheduledId = `scheduled-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    const delivery = SendService.#queueRequest(SendService.#deliveryRequest(req, trackingId))
    const scheduledDocId = await this.scheduledHelper.put({
      id: scheduledId,
      email: delivery,
      sender: senderEmail,
      owner: ownerEmail,
      sendAt: sendAtMs,
      createdAt: Date.now(),
      status: 'scheduled',
    })
    let queueId
    try {
      queueId = await this.outboundQueueService.enqueue(senderEmail, delivery, {
        owner: ownerEmail,
        availableAt: sendAtMs,
        trackingId,
        id: scheduledId,
        scheduledId,
        attachmentEmailId: scheduledId,
      })
      await this.scheduledHelper.update(scheduledDocId, { queueId })
    } catch (error) {
      await this.scheduledHelper.delete(scheduledDocId).catch(() => {})
      throw error
    }
    let stored
    try {
      stored = await this.#storeSentEmail(trackingId, ownerEmail, senderEmail, scheduledId, req)
    } catch (error) {
      const [queueDocId] = await this.outboxHelper.findById(queueId)
      if (queueDocId) await this.outboxHelper.delete(queueDocId).catch(() => {})
      await this.scheduledHelper.delete(scheduledDocId).catch(() => {})
      throw error
    }
    return { scheduled: true, id: scheduledId, threadId: stored.threadId }
  }

  /** Replace a pending scheduled message while retaining its canonical id. */
  async updateScheduled(scheduledDocId, entry, queueDocId, ownerEmail, senderEmail, req, sendAtMs) {
    const delivery = SendService.#queueRequest(SendService.#deliveryRequest(req, entry.email.trackingId))
    await this.outboxHelper.update(queueDocId, {
      email: delivery,
      sender: senderEmail,
      owner: ownerEmail,
      expiresAt: sendAtMs,
      nextAttempt: sendAtMs,
      attempts: 0,
      status: 'queued',
      lastError: '',
      claimedAt: 0,
      attachmentEmailId: entry.id,
    })
    await this.scheduledHelper.update(scheduledDocId, {
      email: delivery,
      sender: senderEmail,
      sendAt: sendAtMs,
      updatedAt: Date.now(),
      status: 'scheduled',
    })
    const stored = await this.#replaceSentEmail(entry.email.trackingId, ownerEmail, senderEmail, entry.id, req)
    return { scheduled: true, id: entry.id, threadId: stored.threadId, sendAt: new Date(sendAtMs).toISOString() }
  }

  /** Replace the local management and Sent projections after the Go-owned
   * provider journal has accepted an in-place scheduled-send edit. */
  async updateExternalScheduled(scheduledDocId, entry, ownerEmail, senderEmail, req, sendAtMs, trackingId = entry.email.trackingId) {
    const delivery = SendService.#queueRequest(SendService.#deliveryRequest(req, trackingId))
    await this.scheduledHelper.update(scheduledDocId, {
      email: delivery,
      sender: senderEmail,
      sendAt: sendAtMs,
      updatedAt: Date.now(),
      status: 'scheduled',
    })
    const stored = await this.#replaceSentEmail(trackingId, ownerEmail, senderEmail, entry.id, req)
    return { scheduled: true, id: entry.id, threadId: stored.threadId, sendAt: new Date(sendAtMs).toISOString() }
  }

  /** Attach source-archive rollback state before the route moves any messages. */
  async attachArchiveRollback(undoId, archiveStateJson) {
    const [docId, entry] = await this.outboxHelper.findById(undoId)
    if (!docId || !entry) throw new Error('Undo delivery is no longer available')
    await this.outboxHelper.update(docId, { archiveStateJson })
  }

  /** Update the Sent representation, MIME source, and attachment bucket set. */
  async #replaceSentEmail(trackingId, ownerEmail, senderEmail, messageId, req) {
    const [docId] = await this.emailsHelper.findById(messageId)
    const domain = ownerEmail.split('@')[1] || ''
    const receivedAt = new Date()
    const threadMetadata = Threading.threadMetadataFromOutbound(req, messageId, domain)
    if (!docId) return await this.#storeSentEmail(trackingId, ownerEmail, senderEmail, messageId, req)
    await this.emailsHelper.update(docId, {
      ownerEmail,
      recipient: req.to.join(', '),
      sender: senderEmail,
      replyTo: req.replyTo ?? [],
      to: req.to.map(address => address.toLowerCase()),
      cc: (req.cc ?? []).map(address => address.toLowerCase()),
      bcc: (req.bcc ?? []).map(address => address.toLowerCase()),
      subject: req.subject,
      body: req.text || SendService.#stripHtml(req.html || '') || '',
      htmlBody: req.html || '',
      openTracking: req.openTracking === true,
      linkTracking: req.linkTracking === true,
      trackedLinksJson: JSON.stringify(req.linkTracking ? TrackingTokens.listTrackableLinks(req.html || '') : []),
      ...threadMetadata,
    })
    await this.attachmentsHelper.replaceForEmail(messageId, domain, req.attachments ?? [])
    const source = await MessageSource.buildOutboundMessageSource(senderEmail, req, messageId, receivedAt)
    await this.rawMessagesHelper.saveForEmail(messageId, domain, source)
    await this.contactsService?.observeParticipants(ownerEmail, [...req.to, ...(req.cc || []), ...(req.bcc || [])], senderEmail.split('@')[1])
    return threadMetadata
  }

  /**
   * Persist a "Sent" copy so the tracking pixel and click redirects have an
   * email row to update with `openedAt` / `clicks`.
   *
   * @param {string | undefined} trackingId @param {string} ownerEmail @param {string} senderEmail @param {string} messageId @param {SendRequest} req
   */
  async #storeSentEmail(trackingId, ownerEmail, senderEmail, messageId, req) {
    const domain = ownerEmail.split('@')[1] || ''
    const sentFolder = await this.categoryMappingService?.pathFor(ownerEmail, domain, 'sent') || 'sent'
    const receivedAt = new Date()
    const threadMetadata = Threading.threadMetadataFromOutbound(req, messageId, domain)
    await this.emailsHelper.put({
      id: messageId,
      domain,
      ownerEmail,
      recipient: req.to.join(', '),
      sender: senderEmail,
      ...(req.replyTo?.length ? { replyTo: req.replyTo } : {}),
      ...(req.to?.length ? { to: req.to.map(address => address.toLowerCase()) } : {}),
      ...(req.cc?.length ? { cc: req.cc.map(address => address.toLowerCase()) } : {}),
      ...(req.bcc?.length ? { bcc: req.bcc.map(address => address.toLowerCase()) } : {}),
      subject: req.subject,
      body: req.text || SendService.#stripHtml(req.html || '') || '',
      ...(req.html ? { htmlBody: req.html } : {}),
      folder: sentFolder,
      read: true,
      starred: false,
      receivedAt: receivedAt.toISOString(),
      processed: true,
      ...(trackingId ? {
        trackingId,
        openTracking: req.openTracking === true,
        linkTracking: req.linkTracking === true,
        trackedLinksJson: JSON.stringify(req.linkTracking ? TrackingTokens.listTrackableLinks(req.html || '') : []),
        opensJson: '[]',
        clicksJson: '[]',
        openCount: 0,
        clickCount: 0,
      } : {}),
      ...threadMetadata,
    })
    if (req.attachments?.length) {
      await this.attachmentsHelper.saveForEmail(messageId, domain, req.attachments)
    }
    const source = await MessageSource.buildOutboundMessageSource(senderEmail, req, messageId, receivedAt)
    await this.rawMessagesHelper.saveForEmail(messageId, domain, source)
    await this.contactsService?.observeParticipants(ownerEmail, [
      ...(req.to || []), ...(req.cc || []), ...(req.bcc || []),
    ], senderEmail.split('@')[1])
    return threadMetadata
  }

  /** @param {SendRequest} req @param {string | undefined} trackingId @returns {SendRequest} */
  static #deliveryRequest(req, trackingId) {
    return {
      ...req,
      ...(trackingId ? { trackingId } : {}),
    }
  }

  /** Queue rows cannot embed attachment objects; those live in FYLO buckets. */
  static #queueRequest(req) {
    const { attachments: _attachments, ...delivery } = req
    return delivery
  }

  /** @param {string} html */
  static #stripHtml(html) {
    return String(html || '')
      .replace(/<style\b[\s\S]*?<\/style>/gi, ' ')
      .replace(/<script\b[\s\S]*?<\/script>/gi, ' ')
      .replace(/<[^>]+>/g, ' ')
      .replace(/&nbsp;/gi, ' ')
      .replace(/&amp;/gi, '&')
      .replace(/&lt;/gi, '<')
      .replace(/&gt;/gi, '>')
      .replace(/&quot;/gi, '"')
      .replace(/&#39;/g, "'")
      .replace(/\s+/g, ' ')
      .trim()
  }
}
