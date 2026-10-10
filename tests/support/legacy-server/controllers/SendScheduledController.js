import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { ScheduledSends } from '../shared/delivery/ScheduledSends.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/send/scheduled")
@SignedIn
export class SendScheduledController {
  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {FollowUpReminderService} */
  @Needs
  followUpReminderService

  /** @type {FollowUpRemindersHelper} */
  @Needs
  followUpRemindersHelper

  /** @type {OutboxHelper} */
  @Needs
  outboxHelper

  /** @type {ScheduledHelper} */
  @Needs
  scheduledHelper

  /** @type {SendService} */
  @Needs
  sendService

  /**
 * DELETE /send/scheduled
 * @param {object} params
 * @param {{ id: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth, context }) => {
    const id = body?.id
    if (!id || typeof id !== 'string') return Respond.r400('id is required')

    const app = this
    const record = await ScheduledSends.findOwnedScheduled(app, id, auth.email)
    if (!record) return Respond.r404('Scheduled email not found')
  	let result
  	try {
  		result = record.entry.transport === 'external'
  			? await ScheduledSends.cancelExternalScheduled(app, record, auth.email, ExternalAccounts.bearerToken(context))
  			: await ScheduledSends.cancelScheduled(app, record, auth.email)
  	} catch (error) {
  		return ExternalAccounts.accountFailure(error)
  	}
    return result.conflict ? Respond.r409('Scheduled email is currently being delivered') : result
    })
  }

  /**
 * GET /send/scheduled
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth, context }) => {
    const app = this
    const records = await app.scheduledHelper.listForOwner(auth.email)
    const list = []
    for (const entry of Object.values(records)) {
      const record = await ScheduledSends.findOwnedScheduled(app, entry.id, auth.email)
  		if (!record) continue
  		if (record.entry.transport === 'external') {
  			try {
  				const submission = await ExternalAccounts.submission(ExternalAccounts.bearerToken(context), record.entry.externalAccountId, record.entry.externalSubmissionId)
  				if (submission.state === 'sent' || submission.state === 'cancelled') {
  					await app.scheduledHelper.delete(record.docId)
  					continue
  				}
  				list.push(await ScheduledSends.presentScheduled(app, record, { externalSubmission: submission }))
  			} catch (error) {
  				return ExternalAccounts.accountFailure(error)
  			}
  			continue
  		}
  		list.push(await ScheduledSends.presentScheduled(app, record))
    }
    return list.sort((a, b) => a.sendAt.localeCompare(b.sendAt))
    })
  }
}
