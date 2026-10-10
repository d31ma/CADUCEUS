import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

export class Handler {
  /**
 * DELETE /users/_id
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN, domain: r => r.body?.domains?.[0] })
  static async DELETE(/** @type {any} */ { paths, auth }) {
  const email = paths?.id
  if (!email) return Respond.r400('email required')

  const app = await getApp()
  const [docId] = await app.users.findByEmail(email)
  if (!docId) return Respond.r404('User not found')

  await app.users.delete(docId)
  return { deleted: email }
}
}
