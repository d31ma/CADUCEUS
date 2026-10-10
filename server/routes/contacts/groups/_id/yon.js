import { Guards } from '../../../../middleware/guards.js'
import { Respond } from '../../../../platform/http/respond.js'
import { getApp } from '../../../../app.js'

export class Handler {
  @Guards.auth()
  static async PUT(/** @type {any} */ { paths, body, auth }) {
    try {
      return await (await getApp()).contacts.updateGroup(String(paths?.id || ''), auth.email, body?.name) ?? Respond.r404('Contact group not found')
    } catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
  }

  @Guards.auth()
  static async DELETE(/** @type {any} */ { paths, auth }) {
    if (!await (await getApp()).contacts.deleteGroup(String(paths?.id || ''), auth.email)) return Respond.r404('Contact group not found')
    return { deleted: true }
  }
}
