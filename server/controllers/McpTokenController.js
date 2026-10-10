import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

@RequestRoute("/mcp/token")
@Public
export class McpTokenController {
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  @PostRoute
  async post(/** @type {any} */ { auth }) {
    const app = await getApp()
    return await app.mcpAccess.rotate(auth.email) ?? Respond.r404('User not found')
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  @DeleteRoute
  async delete(/** @type {any} */ { auth }) {
    const app = await getApp()
    return { revoked: await app.mcpAccess.revoke(auth.email) }
  }
}
