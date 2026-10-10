import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { MailValidation } from '../../features/delivery/services/mail-validation.js'
import { SendService } from '../../features/delivery/services/send.js'
import { createMessageId } from '../../features/mailbox/services/threading.js'
import { ownedSenderAddresses, resolveOwnedSender } from '../../features/users/services/sender-addresses.js'
import { ConversationArchiveError } from '../../features/mailbox/services/conversation-archive.js'
import { buildOutboundMessageSource } from '../../features/mailbox/services/message-source.js'
import { externalDeliveryRequest, externalUndoToken } from '../../features/delivery/services/external-submission.js'
import { parseExternalMessageId } from '../../features/mailbox/services/external-message.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../platform/accounts/bridge.js'
import { SendIdempotencyConflict } from '../../features/delivery/services/send-idempotency.js'

const SEND_LIMIT_PER_HOUR = 60
const HOUR_MS = 60 * 60 * 1000

@RequestRoute("/send")
@Public
export class SendController {
  /**
   * POST /send
   * @param {object} params
   * @param {{ to: string[], subject: string, cc?: string[], bcc?: string[], replyTo?: string[], text?: string, html?: string, delayMs?: number, sendAt?: string, openTracking?: boolean, linkTracking?: boolean, attachments?: unknown, archive?: boolean, archiveSourceId?: string, idempotencyKey?: string }} params.body
   * @param {{ bearer?: { token: string }, ipAddress: string, requestId: string }} params.context
   * @returns {Promise<Record<string, unknown>>}
   */
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, context, auth }) {
    const app = await getApp()

    let idempotencyKey = ''
    let requestHash = ''
    if (body?.idempotencyKey != null) {
      if (typeof body.idempotencyKey !== 'string' || !/^[^\r\n\0]{1,128}$/.test(body.idempotencyKey)) {
        return Respond.r400('idempotencyKey must be 1-128 safe characters')
      }
      idempotencyKey = body.idempotencyKey
      requestHash = app.sendIdempotency.hash(body)
      try {
        const existing = await app.sendIdempotency.inspect(auth.email, idempotencyKey, requestHash)
        if (existing.kind === 'replay') return existing.result
      } catch (error) {
        if (error instanceof SendIdempotencyConflict) return Respond.r409(error.message)
        throw error
      }
    }

    const limit = await app.rateLimit.check(['send', auth.email, context.ipAddress], SEND_LIMIT_PER_HOUR, HOUR_MS)
    if (!limit.allowed) return Respond.r429('Too many send attempts', limit.retryAfterSeconds)

    const parsed = MailValidation.validateSendRequest(body)
    if (!parsed.ok) return Respond.r400(parsed.error)
    const parsedAttachments = MailValidation.validateSendAttachments(body?.attachments)
    if (!parsedAttachments.ok) return Respond.r400(parsedAttachments.error)
    const senderAddresses = await ownedSenderAddresses(app.users, auth.email)
    let senderEmail = resolveOwnedSender(parsed.value.from, senderAddresses, auth.email)
    let externalAccount
    if (!senderEmail && parsed.value.from) {
      try {
        const accounts = await ExternalAccounts.list(bearerToken(context))
        externalAccount = accounts.find(account => account.mailboxAddress === parsed.value.from && account.capabilities?.send === true
          && ['ready', 'syncing', 'idle', 'degraded'].includes(account.state))
        senderEmail = externalAccount?.mailboxAddress
      } catch (error) {
        return accountFailure(error)
      }
    }
    if (!senderEmail) return Respond.r403('From address is not owned by this mailbox')
		const selectedSender = /** @type {string} */ (senderEmail)

    /** @type {SendRequest} */
    const req = {
      ...parsed.value,
      messageId: createMessageId(selectedSender.split('@')[1] || 'localhost'),
      ...(parsedAttachments.value.length ? { attachments: parsedAttachments.value } : {}),
    }
		const outboundMessageId = /** @type {string} */ (req.messageId)

    const suppressed = await app.suppressed.asSet()
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
      externalArchiveSourceId = parseExternalMessageId(body.archiveSourceId.trim()) ? body.archiveSourceId.trim() : ''
      if (!externalArchiveSourceId) {
        try {
          archivePlan = await app.conversationArchive.plan({
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
    const trackingId = req.openTracking || req.linkTracking ? Bun.randomUUIDv7() : undefined

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
      const reminder = await app.reminders.schedule({
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
			const started = await app.sendIdempotency.begin(auth.email, idempotencyKey, requestHash)
			if (started.kind === 'replay') return started.result
			if (started.kind === 'started') sendRequestDocumentId = started.documentId
		} catch (error) {
			if (error instanceof SendIdempotencyConflict) return Respond.r409(error.message)
			throw error
		}
	}
	const accept = async result => sendRequestDocumentId
		? await app.sendIdempotency.accept(sendRequestDocumentId, result)
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
					submission = await ExternalAccounts.beginSubmissionBatch(bearerToken(context), externalAccount.id, {
						idempotencyKey: providerIdempotencyKey, envelopeFrom: selectedSender, recipients: envelopeRecipients, notBefore,
					})
					if (submission.state === 'building') {
						for (let index = Number(submission.deliveryCount || 0); index < envelopeRecipients.length; index++) {
							const recipient = envelopeRecipients[index]
							const providerRequest = externalDeliveryRequest(req, trackingId, recipient)
							const source = await buildOutboundMessageSource(selectedSender, providerRequest, outboundMessageId)
							submission = await ExternalAccounts.appendSubmissionBatch(bearerToken(context), externalAccount.id, {
								submissionId: submission.id, revision: submission.revision, index, recipient,
								rawRFC822Base64: source.toString('base64'),
							})
						}
						submission = await ExternalAccounts.commitSubmissionBatch(bearerToken(context), externalAccount.id, {
							submissionId: submission.id, revision: submission.revision,
						})
					}
					if (!['queued', 'applying', 'sent', 'ambiguous'].includes(submission.state)) {
						throw new Error('Connected-account tracking batch is not runnable')
					}
				} else {
					const providerRequest = externalDeliveryRequest(req, trackingId, envelopeRecipients[0])
					const source = await buildOutboundMessageSource(selectedSender, providerRequest, outboundMessageId)
					submission = await ExternalAccounts.submit(bearerToken(context), externalAccount.id, {
						idempotencyKey: providerIdempotencyKey,
						envelopeFrom: selectedSender,
						recipients: envelopeRecipients,
						rawRFC822Base64: source.toString('base64'),
						notBefore,
					})
				}
			if (sendAtMs != null) {
				const { attachments: _attachments, ...scheduledEmail } = req
				await app.scheduled.put({
					id: stored.messageId, email: { ...scheduledEmail, ...(trackingId ? { trackingId } : {}) }, sender: selectedSender, owner: auth.email,
					sendAt: sendAtMs, createdAt: Date.now(), status: 'scheduled', transport: 'external',
					externalAccountId: externalAccount.id, externalSubmissionId: submission.id,
				})
			}
			/** @type {Record<string, any>} */
			let accepted = sendAtMs != null
				? { scheduled: true, id: stored.messageId, messageId: stored.messageId, threadId: stored.threadId, externalAccountId: externalAccount.id }
				: delayMs > 0
					? { sent: true, queued: true, messageId: stored.messageId, threadId: stored.threadId, undoId: externalUndoToken(externalAccount.id, submission.id, stored.messageId) }
					: { queued: true, queueId: submission.id, messageId: stored.messageId, threadId: stored.threadId, externalAccountId: externalAccount.id }
			if (!externalArchiveSourceId) return await accept(await withReminder(accepted))
			try {
				const archived = await app.mailActions.apply({
					ids: [externalArchiveSourceId], patch: { folder: 'archive' }, ownerEmail: auth.email,
					domains: auth.domains, providerToken: bearerToken(context),
				})
				if (delayMs > 0 && submission?.id) {
					const undoId = externalUndoToken(externalAccount.id, submission.id, stored.messageId, archived.actionId)
					if (!undoId) throw new Error('Connected-account archive undo identity is invalid')
					accepted = { ...accepted, undoId }
				}
				return await accept({ ...await withReminder(accepted), archived: true, archivedCount: archived.count })
			} catch (error) {
				return await accept({ ...await withReminder(accepted), archived: false, archivedCount: 0, archiveError: error instanceof Error ? error.message : 'Source conversation could not be archived' })
			}
		} catch (error) {
			if (submission?.id) {
				await ExternalAccounts.cancelSubmission(bearerToken(context), externalAccount.id, submission.id).catch(() => {})
			}
			if (stored?.messageId) await app.sendService.rollbackExternalAccepted(stored.messageId).catch(() => {})
			if (sendRequestDocumentId) await app.sendIdempotency.markAmbiguous(sendRequestDocumentId).catch(() => {})
			return accountFailure(error)
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
				await app.sendService.attachArchiveRollback(accepted.undoId, app.conversationArchive.serialize(archivePlan))
			}
			finalResult = { ...result, ...await app.conversationArchive.archive(archivePlan, auth.email) }
		} catch (error) {
			console.error('[send] message accepted but source archive failed:', error)
			finalResult = { ...result, archived: false, archivedCount: 0, archiveError: 'Source conversation could not be archived' }
		}
		return await accept(finalResult)
	} catch (error) {
		if (sendRequestDocumentId) await app.sendIdempotency.markAmbiguous(sendRequestDocumentId).catch(() => {})
		throw error
	}
  }
}
