import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { MailActionError, MailActionService } from '../../../features/mailbox/services/mail-actions.js'
import { bearerToken } from '../../../platform/accounts/bridge.js'

@RequestRoute("/inbox/actions")
@Public
export class InboxActionsController {
  /** @param {{body?:unknown,auth:any,context?:any}} params */
  @Guards.auth()
  @PostRoute
  async post({ body, auth, context }) {
    let input
    try { input = MailActionService.parseInput(body) } catch (error) { return InboxActionsController._respondError(error) }
    try {
      const app = await getApp()
      const result = await app.mailActions.apply({
        ...input, ownerEmail: auth.email, domains: auth.domains, providerToken: bearerToken(context),
      })
      return await InboxActionsController._present(app, auth.email, result)
    } catch (error) {
      return InboxActionsController._respondError(error)
    }
  }

  static async _present(app, ownerEmail, result) {
    return {
      ...result,
      expiresAt: new Date(result.expiresAt).toISOString(),
      emails: await Promise.all(result.emails.map(email => app.categoryMappings.presentEmail(ownerEmail, email))),
    }
  }

  static _respondError(error) {
    if (!(error instanceof MailActionError)) throw error
    if (error.status === 404) return Respond.r404(error.message)
    if (error.status === 409) return Respond.r409(error.message)
    if (error.status === 503) return Respond.r503(error.message)
    return Respond.r400(error.message)
  }
}
