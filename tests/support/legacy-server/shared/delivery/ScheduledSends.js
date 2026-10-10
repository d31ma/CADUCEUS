import { Buffer } from 'node:buffer'
import { OutboundQueueRules } from './OutboundQueueRules.js'
import { ExternalAccounts } from '../accounts/ExternalAccounts.js'

export class ScheduledSends {
  /** Exact-owner lookup for scheduled-management routes. */
  static async findOwnedScheduled(app, id, ownerEmail) {
    const [docId, entry] = await app.scheduledHelper.findById(id)
    if (!docId || !entry || (entry.owner ?? entry.sender) !== ownerEmail) return null
  	const [queueDocId, queue] = entry.transport === 'external' ? [null, null] : await app.outboxHelper.findById(entry.queueId ?? entry.id)
    return { docId, entry, queueDocId, queue }
  }

  /** Bounded management representation; attachment bytes are opt-in for editing. */
  static async presentScheduled(app, record, { includeContent = false, externalSubmission = null } = {}) {
    const { entry, queue } = record
  	const submission = /** @type {any} */ (externalSubmission)
    const reminder = await app.followUpRemindersHelper.findActiveForEmail(entry.owner ?? entry.sender, entry.id)
    const attachmentList = await ScheduledSends.loadStoredAttachments(app, entry.id, includeContent)
    return {
      id: entry.id,
      from: entry.sender,
      to: entry.email.to,
      cc: entry.email.cc ?? [],
      bcc: entry.email.bcc ?? [],
      replyTo: entry.email.replyTo ?? [],
      subject: entry.email.subject,
      text: entry.email.text ?? '',
      html: entry.email.html ?? '',
      inReplyTo: entry.email.inReplyTo ?? '',
      references: entry.email.references ?? [],
      openTracking: entry.email.openTracking === true,
      linkTracking: entry.email.linkTracking === true,
      attachments: attachmentList,
      sendAt: new Date(entry.sendAt).toISOString(),
      createdAt: new Date(entry.createdAt).toISOString(),
      ...(entry.updatedAt ? { updatedAt: new Date(entry.updatedAt).toISOString() } : {}),
  		status: entry.transport === 'external'
  			? submission?.state === 'failed' || submission?.state === 'ambiguous' ? submission.state : 'scheduled'
  			: queue?.status ?? (entry.status === 'scheduled' || !entry.status ? 'reconciling' : entry.status),
  		attempts: submission?.attempts ?? queue?.attempts ?? 0,
  		nextAttempt: submission?.notBefore ?? (queue ? new Date(queue.nextAttempt ?? queue.expiresAt).toISOString() : null),
  		lastError: String(submission?.lastCode || queue?.lastError || '').slice(0, 500),
      remindAt: reminder ? new Date(reminder.remindAt).toISOString() : null,
    }
  }

  /** Cancel the Go-owned journal before deleting its local management copy. */
  static async cancelExternalScheduled(app, record, ownerEmail, token) {
  	if (record.entry.transport !== 'external' || !record.entry.externalAccountId || !record.entry.externalSubmissionId) {
  		return { conflict: true }
  	}
  	await ExternalAccounts.cancelSubmission(token, record.entry.externalAccountId, record.entry.externalSubmissionId)
  	await app.scheduledHelper.delete(record.docId)
  	await app.followUpReminderService.dismissForEmail(record.entry.id, ownerEmail)
  	await app.sendService.rollbackExternalAccepted(record.entry.id)
  	return { cancelled: true }
  }

  /** Read one normalized bucket-backed attachment set for editing or delivery. */
  static async loadStoredAttachments(app, emailId, includeContent = true) {
    const records = await app.attachmentsHelper.list(emailId)
    const result = []
    for (const record of records) {
      const contentBase64 = includeContent
        ? Buffer.from(await app.attachmentsHelper.readContent(record)).toString('base64')
        : undefined
      result.push({
        filename: record.filename,
        contentType: record.contentType,
        size: record.size,
        disposition: record.disposition ?? 'attachment',
        ...(record.contentId ? { contentId: record.contentId } : {}),
        ...(contentBase64 ? { contentBase64 } : {}),
      })
    }
    return result
  }

  /** Cancel queue work first so a deleted management row can never still send. */
  static async cancelScheduled(app, record, ownerEmail, now = Date.now()) {
    if (record.queue?.status === 'sending' && (record.queue.claimedAt ?? 0) > now - OutboundQueueRules.DELIVERY_LEASE_MS) {
      return { conflict: true }
    }
    if (record.queueDocId) await app.outboxHelper.delete(record.queueDocId)
    await app.scheduledHelper.delete(record.docId)
    await app.followUpReminderService.dismissForEmail(record.entry.id, ownerEmail)
    const [emailDocId] = await app.emailsHelper.findById(record.entry.id)
    if (emailDocId) await app.emailsHelper.delete(emailDocId, record.entry.id)
    return { cancelled: true }
  }
}
