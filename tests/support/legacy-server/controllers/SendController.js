import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { MailValidation } from '../shared/delivery/MailValidation.js'
import { Threading } from '../shared/mailbox/Threading.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'
import { ConversationArchiveError } from '../shared/errors/ConversationArchiveError.js'
import { MessageSource } from '../shared/mailbox/MessageSource.js'
import { ExternalSubmission } from '../shared/delivery/ExternalSubmission.js'
import { ExternalMessage } from '../shared/mailbox/ExternalMessage.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'
import { SendIdempotencyConflict } from '../shared/errors/SendIdempotencyConflict.js'
import { Uuid } from '../shared/runtime/Uuid.js'
/** @import { SendRequest } from '../models/SendRequest.js' */

@RequestRoute("/send")
@SignedIn
export class SendController {
  /** @type {ConversationArchiveService} */
  @Needs
  conversationArchiveService

  /** @type {FollowUpReminderService} */
  @Needs
  followUpReminderService

  /** @type {MailActionService} */
  @Needs
  mailActionService

  /** @type {RateLimitService} */
  @Needs
  rateLimitService

  /** @type {ScheduledHelper} */
  @Needs
  scheduledHelper

  /** @type {SendIdempotencyService} */
  @Needs
  sendIdempotencyService

  /** @type {SendService} */
  @Needs
  sendService

  /** @type {SuppressedHelper} */
  @Needs
  suppressedHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  static SEND_LIMIT_PER_HOUR = 60

  static HOUR_MS = 60 * 60 * 1000

  /**
   * POST /send
   * @param {object} params
   * @param {{ to: string[], subject: string, cc?: string[], bcc?: string[], replyTo?: string[], text?: string, html?: string, delayMs?: number, sendAt?: string, openTracking?: boolean, linkTracking?: boolean, attachments?: unknown, archive?: boolean, archiveSourceId?: string, idempotencyKey?: string }} params.body
   * @param {{ bearer?: { token: string }, ipAddress: string, requestId: string }} params.context
   * @returns {Promise<Record<string, unknown>>}
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, context, auth }) => {
      const app = this

      let idempotencyKey = ''
      let requestHash = ''
      if (body?.idempotencyKey != null) {
        if (typeof body.idempotencyKey !== 'string' || !/^[^\r\n\0]{1,128}$/.test(body.idempotencyKey)) {
          return Respond.r400('idempotencyKey must be 1-128 safe characters')
        }
        idempotencyKey = body.idempotencyKey
        requestHash = app.sendIdempotencyService.hash(body)
        try {
          const existing = await app.sendIdempotencyService.inspect(auth.email, idempotencyKey, requestHash)
          if (existing.kind === 'replay') return existing.result
        } catch (error) {
          if (error instanceof SendIdempotencyConflict) return Respond.r409(error.message)
          throw error
        }
      }

      const limit = await app.rateLimitService.check(['send', auth.email, context.ipAddress], SendController.SEND_LIMIT_PER_HOUR, SendController.HOUR_MS)
      if (!limit.allowed) return Respond.r429('Too many send attempts', limit.retryAfterSeconds)

      const parsed = MailValidation.validateSendRequest(body)
      if (!parsed.ok) return Respond.r400(parsed.error)
      const parsedAttachments = MailValidation.validateSendAttachments(body?.attachments)
      if (!parsedAttachments.ok) return Respond.r400(parsedAttachments.error)
      const senderAddresses = await SenderAddresses.ownedSenderAddresses(app.usersHelper, auth.email)
      let senderEmail = SenderAddresses.resolveOwnedSender(parsed.value.from, senderAddresses, auth.email)
      let externalAccount
      if (!senderEmail && parsed.value.from) {
        try {
          const accounts = await ExternalAccounts.list(ExternalAccounts.bearerToken(context))
          externalAccount = accounts.find(account => account.mailboxAddress === parsed.value.from && account.capabilities?.send === true
            && ['ready', 'syncing', 'idle', 'degraded'].includes(account.state))
          senderEmail = externalAccount?.mailboxAddress
        } catch (error) {
          return ExternalAccounts.accountFailure(error)
        }
      }
      if (!senderEmail) return Respond.r403('From address is not owned by this mailbox')
  		const selectedSender = /** @type {string} */ (senderEmail)

      /** @type {SendRequest} */
      const req = {
        ...parsed.value,
        messageId: Threading.createMessageId(selectedSender.split('@')[1] || 'localhost'),
        ...(parsedAttachments.value.length ? { attachments: parsedAttachments.value } : {}),
      }
  		const outboundMessageId = /** @type {string} */ (req.messageId)

      const suppressed = await app.suppressedHelper.asSet()
      const recipients = [...req.to, ...(req.cc ?? []), ...(req.bcc ?? [])]
      const blocked = recipients.filter(addr => suppressed.has(addr))
      if (blocked.length > 0) return Respond.r422('Recipients are suppressed', blocked)

      let archivePlan
      let externalArchiveSourceId = ''
      if (body?.archive != null && typeof body.archive !== 'boolean') return Respond.r400('archive must be a boolean')
      if (body?.archiveSourceId != null && body.archive !== true) return Respond.r400('archiveSourceId requires archive: true')
      if (body?.archive === true) {
        if (typeof body.archiveSourceId !== 'string' || !body.archiveSourceId.trim()) {
          return Respond.r400('archiveSourceId is required for Send & Archive')
        }
        if (body.sendAt != null) return Respond.r400('Send & Archive cannot be combined with scheduled delivery')
        externalArchiveSourceId = ExternalMessage.parseExternalMessageId(body.archiveSourceId.trim()) ? body.archiveSourceId.trim() : ''
        if (!externalArchiveSourceId) {
          try {
            archivePlan = await app.conversationArchiveService.plan({
              sourceEmailId: body.archiveSourceId.trim(), ownerEmail: auth.email, domains: auth.domains,
            })
          } catch (error) {
            if (error instanceof ConversationArchiveError && error.status === 404) return Respond.r404(error.message)
            if (error instanceof ConversationArchiveError && error.status === 409) return Respond.r409(error.message)
            throw error
          }
        }
      }

      // The server is the sole issuer of tracking identities. Never accept a
      // caller-selected id that could collide with another mailbox's history.
      const trackingId = req.openTracking || req.linkTracking ? Uuid.v7() : undefined

      let remindAtMs
      if (body?.remindAt != null) {
        if (typeof body.remindAt !== 'string') return Respond.r400('remindAt must be an ISO datetime')
        remindAtMs = Date.parse(body.remindAt)
        if (isNaN(remindAtMs)) return Respond.r400('remindAt must be a valid ISO datetime')
        if (remindAtMs <= Date.now()) return Respond.r400('remindAt must be in the future')
      }

      const withReminder = async result => {
        if (remindAtMs == null) return result
        const emailId = result.messageId ?? result.undoId ?? result.id
        const reminder = await app.followUpReminderService.schedule({
          ownerEmail: auth.email,
          emailId,
          recipient: req.to.join(', '),
          subject: req.subject,
          threadId: result.threadId,
          remindAt: remindAtMs,
        })
        return { ...result, reminderId: reminder.id }
      }

      const sendAt = body?.sendAt
      let sendAtMs
      if (sendAt != null) {
  		if (typeof sendAt !== 'string') return Respond.r400('sendAt must be an ISO datetime')
  		sendAtMs = Date.parse(sendAt)
        if (isNaN(sendAtMs)) return Respond.r400('sendAt must be a valid ISO datetime')
        if (sendAtMs <= Date.now()) return Respond.r400('sendAt must be in the future')
        if (remindAtMs != null && remindAtMs <= sendAtMs) {
          return Respond.r400('remindAt must be after sendAt')
        }
      }

      const delayMs = typeof body.delayMs === 'number' && body.delayMs > 0 ? body.delayMs : 0
  	let sendRequestDocumentId = ''
  	if (idempotencyKey) {
  		try {
  			const started = await app.sendIdempotencyService.begin(auth.email, idempotencyKey, requestHash)
  			if (started.kind === 'replay') return started.result
  			if (started.kind === 'started') sendRequestDocumentId = started.documentId
  		} catch (error) {
  			if (error instanceof SendIdempotencyConflict) return Respond.r409(error.message)
  			throw error
  		}
  	}
  	const accept = async result => sendRequestDocumentId
  		? await app.sendIdempotencyService.accept(sendRequestDocumentId, result)
  		: result
  	if (externalAccount) {
  		const providerIdempotencyKey = idempotencyKey || `message:${outboundMessageId}`
  			const notBefore = sendAtMs != null ? new Date(sendAtMs).toISOString()
  				: delayMs > 0 ? new Date(Date.now() + delayMs).toISOString() : null
  			const envelopeRecipients = [...new Set(recipients)]
  			let stored
  			let submission
  			try {
  				stored = await app.sendService.recordExternalAccepted(auth.email, selectedSender, req, trackingId)
  				if (trackingId) {
  					submission = await ExternalAccounts.beginSubmissionBatch(ExternalAccounts.bearerToken(context), externalAccount.id, {
  						idempotencyKey: providerIdempotencyKey, envelopeFrom: selectedSender, recipients: envelopeRecipients, notBefore,
  					})
  					if (submission.state === 'building') {
  						for (let index = Number(submission.deliveryCount || 0); index < envelopeRecipients.length; index++) {
  							const recipient = envelopeRecipients[index]
  							const providerRequest = ExternalSubmission.externalDeliveryRequest(req, trackingId, recipient)
  							const source = await MessageSource.buildOutboundMessageSource(selectedSender, providerRequest, outboundMessageId)
  							submission = await ExternalAccounts.appendSubmissionBatch(ExternalAccounts.bearerToken(context), externalAccount.id, {
  								submissionId: submission.id, revision: submission.revision, index, recipient,
  								rawRFC822Base64: source.toString('base64'),
  							})
  						}
  						submission = await ExternalAccounts.commitSubmissionBatch(ExternalAccounts.bearerToken(context), externalAccount.id, {
  							submissionId: submission.id, revision: submission.revision,
  						})
  					}
  					if (!['queued', 'applying', 'sent', 'ambiguous'].includes(submission.state)) {
  						throw new Error('Connected-account tracking batch is not runnable')
  					}
  				} else {
  					const providerRequest = ExternalSubmission.externalDeliveryRequest(req, trackingId, envelopeRecipients[0])
  					const source = await MessageSource.buildOutboundMessageSource(selectedSender, providerRequest, outboundMessageId)
  					submission = await ExternalAccounts.submit(ExternalAccounts.bearerToken(context), externalAccount.id, {
  						idempotencyKey: providerIdempotencyKey,
  						envelopeFrom: selectedSender,
  						recipients: envelopeRecipients,
  						rawRFC822Base64: source.toString('base64'),
  						notBefore,
  					})
  				}
  			if (sendAtMs != null) {
  				const { attachments: _attachments, ...scheduledEmail } = req
  				await app.scheduledHelper.put({
  					id: stored.messageId, email: { ...scheduledEmail, ...(trackingId ? { trackingId } : {}) }, sender: selectedSender, owner: auth.email,
  					sendAt: sendAtMs, createdAt: Date.now(), status: 'scheduled', transport: 'external',
  					externalAccountId: externalAccount.id, externalSubmissionId: submission.id,
  				})
  			}
  			/** @type {Record<string, any>} */
  			let accepted = sendAtMs != null
  				? { scheduled: true, id: stored.messageId, messageId: stored.messageId, threadId: stored.threadId, externalAccountId: externalAccount.id }
  				: delayMs > 0
  					? { sent: true, queued: true, messageId: stored.messageId, threadId: stored.threadId, undoId: ExternalSubmission.externalUndoToken(externalAccount.id, submission.id, stored.messageId) }
  					: { queued: true, queueId: submission.id, messageId: stored.messageId, threadId: stored.threadId, externalAccountId: externalAccount.id }
  			if (!externalArchiveSourceId) return await accept(await withReminder(accepted))
  			try {
  				const archived = await app.mailActionService.apply({
  					ids: [externalArchiveSourceId], patch: { folder: 'archive' }, ownerEmail: auth.email,
  					domains: auth.domains, providerToken: ExternalAccounts.bearerToken(context),
  				})
  				if (delayMs > 0 && submission?.id) {
  					const undoId = ExternalSubmission.externalUndoToken(externalAccount.id, submission.id, stored.messageId, archived.actionId)
  					if (!undoId) throw new Error('Connected-account archive undo identity is invalid')
  					accepted = { ...accepted, undoId }
  				}
  				return await accept({ ...await withReminder(accepted), archived: true, archivedCount: archived.count })
  			} catch (error) {
  				return await accept({ ...await withReminder(accepted), archived: false, archivedCount: 0, archiveError: error instanceof Error ? error.message : 'Source conversation could not be archived' })
  			}
  		} catch (error) {
  			if (submission?.id) {
  				await ExternalAccounts.cancelSubmission(ExternalAccounts.bearerToken(context), externalAccount.id, submission.id).catch(() => {})
  			}
  			if (stored?.messageId) await app.sendService.rollbackExternalAccepted(stored.messageId).catch(() => {})
  			if (sendRequestDocumentId) await app.sendIdempotencyService.markAmbiguous(sendRequestDocumentId).catch(() => {})
  			return ExternalAccounts.accountFailure(error)
  		}
  	}
  	try {
  		if (sendAtMs != null) {
  			return await accept(await withReminder(await app.sendService.schedule(auth.email, selectedSender, req, sendAtMs, trackingId)))
  		}
  		const accepted = delayMs > 0
  			? await app.sendService.delay(auth.email, selectedSender, req, delayMs, trackingId)
  			: await app.sendService.sendNow(auth.email, selectedSender, req, trackingId)
  		const result = await withReminder(accepted)
  		if (!archivePlan) return await accept(result)

  		let finalResult
  		try {
  			if ('undoId' in accepted && accepted.undoId) {
  				await app.sendService.attachArchiveRollback(accepted.undoId, app.conversationArchiveService.serialize(archivePlan))
  			}
  			finalResult = { ...result, ...await app.conversationArchiveService.archive(archivePlan, auth.email) }
  		} catch (error) {
  			console.error('[send] message accepted but source archive failed:', error)
  			finalResult = { ...result, archived: false, archivedCount: 0, archiveError: 'Source conversation could not be archived' }
  		}
  		return await accept(finalResult)
  	} catch (error) {
  		if (sendRequestDocumentId) await app.sendIdempotencyService.markAmbiguous(sendRequestDocumentId).catch(() => {})
  		throw error
  	}
    })
  }
}
