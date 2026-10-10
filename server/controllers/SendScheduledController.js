import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { cancelScheduled, findOwnedScheduled, presentScheduled } from './helpers.js'
import { cancelExternalScheduled } from './helpers.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../platform/accounts/bridge.js'

@RequestRoute("/send/scheduled")
@Public
export class SendScheduledController {
  /**
 * DELETE /send/scheduled
 * @param {object} params
 * @param {{ id: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  @DeleteRoute
  async delete(/** @type {any} */ { body, auth, context }) {
  const id = body?.id
  if (!id || typeof id !== 'string') return Respond.r400('id is required')

  const app = await getApp()
  const record = await findOwnedScheduled(app, id, auth.email)
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
}

  /**
 * GET /send/scheduled
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { auth, context }) {
  const app = await getApp()
  const records = await app.scheduled.listForOwner(auth.email)
  const list = []
  for (const entry of Object.values(records)) {
    const record = await findOwnedScheduled(app, entry.id, auth.email)
		if (!record) continue
		if (record.entry.transport === 'external') {
			try {
				const submission = await ExternalAccounts.submission(bearerToken(context), record.entry.externalAccountId, record.entry.externalSubmissionId)
				if (submission.state === 'sent' || submission.state === 'cancelled') {
					await app.scheduled.delete(record.docId)
					continue
				}
				list.push(await presentScheduled(app, record, { externalSubmission: submission }))
			} catch (error) {
				return accountFailure(error)
			}
			continue
		}
		list.push(await presentScheduled(app, record))
  }
  return list.sort((a, b) => a.sendAt.localeCompare(b.sendAt))
}
}
