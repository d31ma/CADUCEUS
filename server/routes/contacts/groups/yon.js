import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { auth }) {
    return await (await getApp()).contacts.listGroups(auth.email)
  }

  @Guards.auth()
  static async POST(/** @type {any} */ { body, auth }) {
    try { return await (await getApp()).contacts.createGroup(auth.email, body?.name) }
    catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
  }
}
