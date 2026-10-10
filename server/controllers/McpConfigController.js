import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { McpPreferenceError } from '../shared/errors/McpPreferenceError.js'

@RequestRoute("/mcp/config")
@SignedIn
export class McpConfigController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ auth }) => {
      const app = await getApp()
      return await app.mcpAccess.config(auth.email) ?? Respond.r404('User not found')
    })
  }

  @PutRoute
  async put(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
      try {
        const app = await getApp()
        return await app.mcpAccess.updateConfig(auth.email, body) ?? Respond.r404('User not found')
      } catch (error) {
        if (error instanceof McpPreferenceError) {
          return error.forbidden ? Respond.r403(error.message) : Respond.r400(error.message)
        }
        throw error
      }
    })
  }
}
