import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/thread-shares/public/{token}")
@Public
export class ThreadSharesPublicTokenController {
  /** Public bearer-capability view; invalid, expired, and revoked are identical. */
  @GetRoute
  async get(@FromPath token, request) {
    return RouteRequest.handle(request, { paths: { token } }, async ({ paths }) => {
      const app = await getApp()
      return await app.threadSharing.resolve(paths?.token)
        ?? Respond.r404('Shared conversation not found')
    })
  }
}
