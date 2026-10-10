import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { MailActionService } from '../features/mailbox/services/MailActionService.js'
import { MailActionError } from '../shared/errors/MailActionError.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/inbox/actions")
@SignedIn
export class InboxActionsController {
  /** @param {{body?:unknown,auth:any,context?:any}} params */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth, context }) => {
      let input
      try { input = MailActionService.parseInput(body) } catch (error) { return InboxActionsController._respondError(error) }
      try {
        const app = await getApp()
        const result = await app.mailActions.apply({
          ...input, ownerEmail: auth.email, domains: auth.domains, providerToken: ExternalAccounts.bearerToken(context),
        })
        return await InboxActionsController._present(app, auth.email, result)
      } catch (error) {
        return InboxActionsController._respondError(error)
      }
    })
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
