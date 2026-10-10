import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { getApp } from '../../../app.js'

@RequestRoute("/thread-shares/{id}")
@Public
export class ThreadSharesIdController {
  @Guards.auth({ action: Actions.MAILBOX_READ })
  @DeleteRoute
  async delete(/** @type {any} */ { paths, auth }) {
    const id = String(paths?.id || '')
    if (!/^share-[A-Za-z0-9_-]{20,40}$/.test(id)) return Respond.r404('Share not found')
    const app = await getApp()
    return await app.threadSharing.revoke(id, auth.email)
      ? { revoked: id }
      : Respond.r404('Share not found')
  }
}
