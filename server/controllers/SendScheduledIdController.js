import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { MailValidation } from '../features/delivery/services/MailValidation.js'
import { ownedSenderAddresses, resolveOwnedSender } from '../features/users/services/sender-addresses.js'
import { cancelExternalScheduled, cancelScheduled, findOwnedScheduled, loadStoredAttachments, presentScheduled } from '../features/delivery/services/scheduled-sends.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'
import { buildOutboundMessageSource } from '../features/mailbox/services/message-source.js'
import { externalDeliveryRequest } from '../features/delivery/services/external-submission.js'

@RequestRoute("/send/scheduled/{id}")
@SignedIn
export class SendScheduledIdController {
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      const app = await getApp()
      const record = await findOwnedScheduled(app, String(paths?.id || ''), auth.email)
  		if (!record) return Respond.r404('Scheduled email not found')
  		if (record.entry.transport !== 'external') return await presentScheduled(app, record, { includeContent: true })
  		try {
  			const submission = await ExternalAccounts.submission(bearerToken(context), record.entry.externalAccountId, record.entry.externalSubmissionId)
  			if (submission.state === 'sent' || submission.state === 'cancelled') {
  				await app.scheduled.delete(record.docId)
  				return Respond.r404('Scheduled email not found')
  			}
  			return await presentScheduled(app, record, { includeContent: true, externalSubmission: submission })
  		} catch (error) {
  			return accountFailure(error)
  		}
    })
  }

  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth, context }) => {
      const app = await getApp()
      const record = await findOwnedScheduled(app, String(paths?.id || ''), auth.email)
      if (!record) return Respond.r404('Scheduled email not found')
  		const external = record.entry.transport === 'external'
      if (!external && (!record.queueDocId || !record.queue)) return Respond.r409('Scheduled delivery is being reconciled')
      if (!external && record.queue.status === 'sending') return Respond.r409('Scheduled email is currently being delivered')

      const candidate = { ...record.entry.email, ...(body || {}) }
      const parsed = MailValidation.validateSendRequest(candidate)
      if (!parsed.ok) return Respond.r400(parsed.error)
      const attachments = MailValidation.validateSendAttachments(
        Object.hasOwn(body || {}, 'attachments') ? body.attachments : await loadStoredAttachments(app, record.entry.id),
      )
      if (!attachments.ok) return Respond.r400(attachments.error)
  		let senderEmail
  		if (external) {
  			senderEmail = parsed.value.from ?? record.entry.sender
  			if (senderEmail !== record.entry.sender) senderEmail = undefined
  		} else {
  			const addresses = await ownedSenderAddresses(app.users, auth.email)
  			senderEmail = resolveOwnedSender(parsed.value.from ?? record.entry.sender, addresses, auth.email)
  		}
      if (!senderEmail) return Respond.r403('From address is not owned by this mailbox')

      const sendAtMs = body?.sendAt == null ? record.entry.sendAt : Date.parse(body.sendAt)
      if (!Number.isFinite(sendAtMs)) return Respond.r400('sendAt must be a valid ISO datetime')
      if (sendAtMs <= Date.now()) return Respond.r400('sendAt must be in the future')
      const activeReminder = await app.followUpReminders.findActiveForEmail(auth.email, record.entry.id)
      let remindAtMs = activeReminder?.remindAt
      if (body?.remindAt === null) remindAtMs = undefined
      else if (body?.remindAt != null) {
        if (typeof body.remindAt !== 'string') return Respond.r400('remindAt must be an ISO datetime or null')
        remindAtMs = Date.parse(body.remindAt)
        if (!Number.isFinite(remindAtMs)) return Respond.r400('remindAt must be a valid ISO datetime')
      }
      if (remindAtMs != null && remindAtMs <= sendAtMs) return Respond.r400('remindAt must be after sendAt')

      const req = {
        ...parsed.value,
        messageId: record.entry.email.messageId,
        ...(attachments.value.length ? { attachments: attachments.value } : {}),
      }
      const suppressed = await app.suppressed.asSet()
      const blocked = [...req.to, ...(req.cc ?? []), ...(req.bcc ?? [])].filter(address => suppressed.has(address))
      if (blocked.length) return Respond.r422('Recipients are suppressed', blocked)

  		let result
  		if (external) {
  			if (!record.entry.externalAccountId || !record.entry.externalSubmissionId) return Respond.r409('Scheduled delivery is being reconciled')
  			try {
  				const submission = await ExternalAccounts.submission(bearerToken(context), record.entry.externalAccountId, record.entry.externalSubmissionId)
  				if (submission.state !== 'queued' || !Number.isSafeInteger(submission.revision) || submission.revision < 1) {
  					return Respond.r409('Scheduled email is currently being delivered')
  				}
  				const recipients = [...new Set([...req.to, ...(req.cc ?? []), ...(req.bcc ?? [])])]
  				let trackingId = record.entry.email.trackingId
  				if ((req.openTracking || req.linkTracking) && !trackingId) trackingId = Bun.randomUUIDv7()
  				if (!req.openTracking && !req.linkTracking) trackingId = undefined
  				if (Number(submission.deliveryCount || 0) > 0) {
  					let staged
  					try {
  						staged = await ExternalAccounts.beginSubmissionBatchUpdate(bearerToken(context), record.entry.externalAccountId, {
  							submissionId: record.entry.externalSubmissionId,
  							revision: submission.revision,
  							idempotencyKey: submission.idempotencyKey,
  							envelopeFrom: senderEmail,
  							recipients,
  							notBefore: new Date(sendAtMs).toISOString(),
  						})
  						for (let index = Number(staged.deliveryCount || 0); index < recipients.length; index++) {
  							const recipient = recipients[index]
  							const providerRequest = externalDeliveryRequest(req, trackingId, recipient)
  							const source = await buildOutboundMessageSource(senderEmail, providerRequest, record.entry.id)
  							staged = await ExternalAccounts.appendSubmissionBatchUpdate(bearerToken(context), record.entry.externalAccountId, {
  								submissionId: record.entry.externalSubmissionId,
  								revision: staged.revision,
  								index,
  								recipient,
  								rawRFC822Base64: source.toString('base64'),
  							})
  						}
  						await ExternalAccounts.commitSubmissionBatchUpdate(bearerToken(context), record.entry.externalAccountId, {
  							submissionId: record.entry.externalSubmissionId,
  							revision: staged.revision,
  						})
  					} catch (error) {
  						try {
  							const current = await ExternalAccounts.submission(bearerToken(context), record.entry.externalAccountId, record.entry.externalSubmissionId)
  							if (current.state === 'editing') {
  								await ExternalAccounts.abortSubmissionBatchUpdate(bearerToken(context), record.entry.externalAccountId, {
  									submissionId: record.entry.externalSubmissionId,
  									revision: current.revision,
  								})
  							}
  						} catch {}
  						throw error
  					}
  				} else {
  					if (trackingId && recipients.length !== 1) {
  						return Respond.r409('This legacy scheduled message must be recreated to track multiple recipients')
  					}
  					const providerRequest = externalDeliveryRequest(req, trackingId, recipients[0])
  					const source = await buildOutboundMessageSource(senderEmail, providerRequest, record.entry.id)
  					await ExternalAccounts.updateSubmission(bearerToken(context), record.entry.externalAccountId, {
  						submissionId: record.entry.externalSubmissionId,
  						revision: submission.revision,
  						idempotencyKey: submission.idempotencyKey,
  						envelopeFrom: senderEmail,
  						recipients,
  						rawRFC822Base64: source.toString('base64'),
  						notBefore: new Date(sendAtMs).toISOString(),
  					})
  				}
  				result = await app.sendService.updateExternalScheduled(record.docId, record.entry, auth.email, senderEmail, req, sendAtMs, trackingId)
  			} catch (error) {
  				return accountFailure(error)
  			}
  		} else {
  			result = await app.sendService.updateScheduled(
  				record.docId, record.entry, record.queueDocId, auth.email, senderEmail, req, sendAtMs,
  			)
  		}
      if (body?.remindAt === null) await app.reminders.dismissForEmail(record.entry.id, auth.email)
      else if (remindAtMs != null) {
        await app.reminders.schedule({
          ownerEmail: auth.email,
          emailId: record.entry.id,
          recipient: req.to.join(', '),
          subject: req.subject,
          threadId: String(result.threadId || ''),
          remindAt: remindAtMs,
        })
      }
      return result
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      const app = await getApp()
      const record = await findOwnedScheduled(app, String(paths?.id || ''), auth.email)
      if (!record) return Respond.r404('Scheduled email not found')
  		let result
  		try {
  			result = record.entry.transport === 'external'
  				? await cancelExternalScheduled(app, record, auth.email, bearerToken(context))
  				: await cancelScheduled(app, record, auth.email)
  		} catch (error) {
  			return accountFailure(error)
  		}
      return result.conflict ? Respond.r409('Scheduled email is currently being delivered') : result
    })
  }
}
