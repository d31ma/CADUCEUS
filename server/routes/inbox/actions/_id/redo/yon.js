import { Guards } from '../../../../../middleware/guards.js'
import { Respond } from '../../../../../platform/http/respond.js'
import { getApp } from '../../../../../app.js'
import { Handler as MailActions } from '../../yon.js'
import { bearerToken } from '../../../../../platform/accounts/bridge.js'

export class Handler {
  /** @param {{paths?:{id?:string},auth:any,context?:any}} params */
  @Guards.auth()
  static async POST({ paths, auth, context }) {
    if (!paths?.id) return Respond.r404('Mail action not found')
    try {
      const app = await getApp()
      return await MailActions.present(
        app,
        auth.email,
        await app.mailActions.redo(paths.id, auth.email, auth.domains, Date.now(), bearerToken(context)),
      )
    } catch (error) {
      return MailActions.respondError(error)
    }
  }
}
