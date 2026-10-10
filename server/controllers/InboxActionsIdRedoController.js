import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { InboxActionsController as MailActions } from './InboxActionsController.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/inbox/actions/{id}/redo")
@SignedIn
export class InboxActionsIdRedoController {
  /** @param {{paths?:{id?:string},auth:any,context?:any}} params */
  @PostRoute
  async post(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      if (!paths?.id) return Respond.r404('Mail action not found')
      try {
        const app = await getApp()
        return await MailActions._present(
          app,
          auth.email,
          await app.mailActions.redo(paths.id, auth.email, auth.domains, Date.now(), ExternalAccounts.bearerToken(context)),
        )
      } catch (error) {
        return MailActions._respondError(error)
      }
    })
  }
}
