import { Guards } from '../../../../../middleware/guards.js'
import { Respond } from '../../../../../platform/http/respond.js'
import { getApp } from '../../../../../app.js'
import { InboxActionsIdUndoController as MailActions } from '../../yon.js'
import { bearerToken } from '../../../../../platform/accounts/bridge.js'

@RequestRoute("/inbox/actions/{id}/undo")
@Public
export class InboxActionsIdUndoController {
  /** @param {{paths?:{id?:string},auth:any,context?:any}} params */
  @Guards.auth()
  @PostRoute
  async post({ paths, auth, context }) {
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
  }
}
