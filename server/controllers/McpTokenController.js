import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Actions } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/mcp/token")
@SignedIn
export class McpTokenController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_WRITE } }, async ({ auth }) => {
      const app = await getApp()
      return await app.mcpAccess.rotate(auth.email) ?? Respond.r404('User not found')
    })
  }

  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_WRITE } }, async ({ auth }) => {
      const app = await getApp()
      return { revoked: await app.mcpAccess.revoke(auth.email) }
    })
  }
}
