import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/mcp/token")
@SignedIn
export class McpTokenController {
  /** @type {McpAccessService} */
  @Needs
  mcpAccessService

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ auth }) => {
      const app = this
      return await app.mcpAccessService.rotate(auth.email) ?? Respond.r404('User not found')
    })
  }

  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ auth }) => {
      const app = this
      return { revoked: await app.mcpAccessService.revoke(auth.email) }
    })
  }
}
