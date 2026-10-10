import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { parseExternalUndoToken } from '../../../features/delivery/services/external-submission.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../platform/accounts/bridge.js'

@RequestRoute("/send/undo")
@Public
export class SendUndoController {
  /**
 * POST /send/undo
 * @param {object} params
 * @param {{ undoId: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, auth, context }) {
  const undoId = body?.undoId
  if (!undoId || typeof undoId !== 'string') return Respond.r400('undoId is required')

  const app = await getApp()
  const external = parseExternalUndoToken(undoId)
  if (external) {
    const [, email] = await app.emails.findById(external.messageId)
    if (!email || email.ownerEmail !== auth.email) return Respond.r404('Undo ID not found or already processed')
    try {
      await ExternalAccounts.cancelSubmission(bearerToken(context), external.accountId, external.submissionId)
    } catch (error) {
      return accountFailure(error)
    }
    await app.reminders.dismissForEmail(external.messageId, auth.email)
    await app.sendService.rollbackExternalAccepted(external.messageId)
    if (!external.archiveActionId) return { undone: true }
    try {
      const restored = await app.mailActions.undo(external.archiveActionId, auth.email, auth.domains, Date.now(), bearerToken(context))
      return { undone: true, archiveRestored: restored.count }
    } catch (error) {
      return { undone: true, archiveRestored: 0, archiveError: error instanceof Error ? error.message : 'Source conversation could not be restored' }
    }
  }
  const [docId, entry] = await app.outbox.findById(undoId)
  if (!docId || !entry) return Respond.r404('Undo ID not found or already processed')
  if ((entry.owner ?? entry.sender) !== auth.email) return Respond.r404('Undo ID not found or already processed')

  const restored = entry.archiveStateJson
    ? await app.conversationArchive.restore(entry.archiveStateJson, auth.email, auth.domains)
    : 0
  await app.outbox.delete(docId)
  await app.reminders.dismissForEmail(undoId, auth.email)
  const [emailDocId] = await app.emails.findById(undoId)
  if (emailDocId) await app.emails.delete(emailDocId, undoId)
  return { undone: true, ...(entry.archiveStateJson ? { archiveRestored: restored } : {}) }
}
}
