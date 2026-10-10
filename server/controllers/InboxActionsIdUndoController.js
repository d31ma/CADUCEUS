import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { InboxActionsIdUndoController as MailActions } from './InboxActionsController.js'
import { bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/inbox/actions/{id}/undo")
@SignedIn
export class InboxActionsIdUndoController {
  /** @param {{paths?:{id?:string},auth:any,context?:any}} params */
  @PostRoute
  async post(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      if (!paths?.id) return Respond.r404('Mail action not found')
      try {
        const app = await getApp()
        return await MailActions.present(
          app,
          auth.email,
          await app.mailActions.undo(paths.id, auth.email, auth.domains, Date.now(), bearerToken(context)),
        )
      } catch (error) {
        return MailActions.respondError(error)
      }
    })
  }
}
