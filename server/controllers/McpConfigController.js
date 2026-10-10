import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { McpPreferenceError } from '../../../features/mcp/services/access.js'

@RequestRoute("/mcp/config")
@Public
export class McpConfigController {
  @Guards.auth({ action: Actions.MAILBOX_READ })
  @GetRoute
  async get(/** @type {any} */ { auth }) {
    const app = await getApp()
    return await app.mcpAccess.config(auth.email) ?? Respond.r404('User not found')
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  @PutRoute
  async put(/** @type {any} */ { body, auth }) {
    try {
      const app = await getApp()
      return await app.mcpAccess.updateConfig(auth.email, body) ?? Respond.r404('User not found')
    } catch (error) {
      if (error instanceof McpPreferenceError) {
        return error.forbidden ? Respond.r403(error.message) : Respond.r400(error.message)
      }
      throw error
    }
  }
}
