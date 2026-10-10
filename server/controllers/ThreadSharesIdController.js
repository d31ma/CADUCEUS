import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { Actions } from '../platform/auth/Auth.js'
import { getApp } from '../app.js'

@RequestRoute("/thread-shares/{id}")
@SignedIn
export class ThreadSharesIdController {
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Actions.MAILBOX_READ } }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!/^share-[A-Za-z0-9_-]{20,40}$/.test(id)) return Respond.r404('Share not found')
      const app = await getApp()
      return await app.threadSharing.revoke(id, auth.email)
        ? { revoked: id }
        : Respond.r404('Share not found')
    })
  }
}
