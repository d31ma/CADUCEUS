import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { inspectExtensionBundle, presentExtension } from '../../../features/extensions/services/extensions.js'

export class Handler {
  @Guards.auth({ action: Actions.MAILBOX_READ })
  static async POST(/** @type {any} */ { body }) {
    const inspected = inspectExtensionBundle(body?.bundle)
    if (inspected instanceof Error) return Respond.r400(inspected.message)
    return presentExtension({ ...inspected, enabled: false })
  }
}
