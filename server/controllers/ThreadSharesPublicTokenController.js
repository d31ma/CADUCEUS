import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/thread-shares/public/{token}")
@Public
export class ThreadSharesPublicTokenController {
  /** @type {ThreadSharingService} */
  @Needs
  threadSharingService

  /** Public bearer-capability view; invalid, expired, and revoked are identical. */
  @GetRoute
  async get(@FromPath token, request) {
    return RouteRequest.handle(request, { paths: { token } }, async ({ paths }) => {
      const app = this
      return await app.threadSharingService.resolve(paths?.token)
        ?? Respond.r404('Shared conversation not found')
    })
  }
}
