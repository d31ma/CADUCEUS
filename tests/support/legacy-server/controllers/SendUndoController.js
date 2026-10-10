import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { ExternalSubmission } from '../shared/delivery/ExternalSubmission.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/send/undo")
@SignedIn
export class SendUndoController {
  /** @type {ConversationArchiveService} */
  @Needs
  conversationArchiveService

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {FollowUpReminderService} */
  @Needs
  followUpReminderService

  /** @type {MailActionService} */
  @Needs
  mailActionService

  /** @type {OutboxHelper} */
  @Needs
  outboxHelper

  /** @type {SendService} */
  @Needs
  sendService

  /**
 * POST /send/undo
 * @param {object} params
 * @param {{ undoId: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth, context }) => {
    const undoId = body?.undoId
    if (!undoId || typeof undoId !== 'string') return Respond.r400('undoId is required')

    const app = this
    const external = ExternalSubmission.parseExternalUndoToken(undoId)
    if (external) {
      const [, email] = await app.emailsHelper.findById(external.messageId)
      if (!email || email.ownerEmail !== auth.email) return Respond.r404('Undo ID not found or already processed')
      try {
        await ExternalAccounts.cancelSubmission(ExternalAccounts.bearerToken(context), external.accountId, external.submissionId)
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
      await app.followUpReminderService.dismissForEmail(external.messageId, auth.email)
      await app.sendService.rollbackExternalAccepted(external.messageId)
      if (!external.archiveActionId) return { undone: true }
      try {
        const restored = await app.mailActionService.undo(external.archiveActionId, auth.email, auth.domains, Date.now(), ExternalAccounts.bearerToken(context))
        return { undone: true, archiveRestored: restored.count }
      } catch (error) {
        return { undone: true, archiveRestored: 0, archiveError: error instanceof Error ? error.message : 'Source conversation could not be restored' }
      }
    }
    const [docId, entry] = await app.outboxHelper.findById(undoId)
    if (!docId || !entry) return Respond.r404('Undo ID not found or already processed')
    if ((entry.owner ?? entry.sender) !== auth.email) return Respond.r404('Undo ID not found or already processed')

    const restored = entry.archiveStateJson
      ? await app.conversationArchiveService.restore(entry.archiveStateJson, auth.email, auth.domains)
      : 0
    await app.outboxHelper.delete(docId)
    await app.followUpReminderService.dismissForEmail(undoId, auth.email)
    const [emailDocId] = await app.emailsHelper.findById(undoId)
    if (emailDocId) await app.emailsHelper.delete(emailDocId, undoId)
    return { undone: true, ...(entry.archiveStateJson ? { archiveRestored: restored } : {}) }
    })
  }
}
