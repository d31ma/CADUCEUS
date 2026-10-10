import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

export class Handler {
  /**
 * DELETE /suppressed/_address
 * @param {object} params
 * @param {{ address: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN })
  static async DELETE(/** @type {any} */ { paths, auth }) {
  const address = paths?.address
  if (!address) return Respond.r400('address required')

  const app = await getApp()
  await app.suppressed.delete(address)
  return { removed: address }
}
}
