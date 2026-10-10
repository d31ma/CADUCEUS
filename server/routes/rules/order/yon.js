import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

export class Handler {
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async PUT(/** @type {any} */ { body, auth }) {
    const ids = body?.ids
    if (!Array.isArray(ids) || ids.length > 100 || ids.some(id => typeof id !== 'string') || new Set(ids).size !== ids.length) {
      return Respond.r400('ids must be a unique list of rule IDs')
    }
    const app = await getApp()
    if (!await app.rules.reorder(auth.email, ids)) return Respond.r400('ids must include every owned rule exactly once')
    return { ordered: ids.length }
  }
}

