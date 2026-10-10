import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { SnoozeError } from '../../features/mailbox/services/snooze.js'

export class Handler {
  /** @param {{body?:{emailId?:unknown,until?:unknown,label?:unknown,clientRequestId?:unknown},auth:any}} params */
  @Guards.auth()
  static async POST({ body, auth }) {
    if (typeof body?.emailId !== 'string' || !body.emailId) return Respond.r400('emailId is required')
    if (typeof body?.until !== 'string' || !body.until) return Respond.r400('until (ISO datetime) is required')
    if (body.label != null && typeof body.label !== 'string') return Respond.r400('label must be text')
    if (body.clientRequestId != null && (typeof body.clientRequestId !== 'string'
      || !/^[A-Za-z0-9._:-]{1,128}$/.test(body.clientRequestId))) return Respond.r400('clientRequestId is invalid')
    const until = Date.parse(body.until)
    if (!Number.isFinite(until)) return Respond.r400('until must be a valid ISO datetime')
    const app = await getApp()
    try {
      return await app.snoozeService.snoozeThread({
        emailId: body.emailId,
        until,
        ...(typeof body.label === 'string' ? { label: body.label } : {}),
        ...(typeof body.clientRequestId === 'string' ? { clientRequestId: body.clientRequestId } : {}),
        ownerEmail: auth.email,
        domains: auth.domains,
      })
    } catch (error) {
      return Handler.respondError(error)
    }
  }

  /** @param {{auth:any}} params */
  @Guards.auth()
  static async GET({ auth }) {
    const app = await getApp()
    return await app.snoozeService.listActive(auth.email, auth.domains)
  }

  /** @param {unknown} error */
  static respondError(error) {
    if (!(error instanceof SnoozeError)) throw error
    if (error.status === 404) return Respond.r404(error.message)
    if (error.status === 409) return Respond.r409(error.message)
    return Respond.r400(error.message)
  }
}
