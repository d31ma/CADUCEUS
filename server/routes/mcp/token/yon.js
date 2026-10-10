import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

export class Handler {
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async POST(/** @type {any} */ { auth }) {
    const app = await getApp()
    return await app.mcpAccess.rotate(auth.email) ?? Respond.r404('User not found')
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { auth }) {
    const app = await getApp()
    return { revoked: await app.mcpAccess.revoke(auth.email) }
  }
}
