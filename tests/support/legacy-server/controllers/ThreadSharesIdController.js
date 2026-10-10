import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'

@RequestRoute("/thread-shares/{id}")
@SignedIn
export class ThreadSharesIdController {
  /** @type {ThreadSharingService} */
  @Needs
  threadSharingService

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!/^share-[A-Za-z0-9_-]{20,40}$/.test(id)) return Respond.r404('Share not found')
      const app = this
      return await app.threadSharingService.revoke(id, auth.email)
        ? { revoked: id }
        : Respond.r404('Share not found')
    })
  }
}
