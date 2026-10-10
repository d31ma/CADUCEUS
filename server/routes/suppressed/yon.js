import { Guards } from '../../middleware/guards.js'
import { Actions } from '../../platform/auth/bridge.js'
import { getApp } from '../../app.js'
import { Respond } from '../../platform/http/respond.js'

export class Handler {
  /**
 * GET /suppressed
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN })
  static async GET(/** @type {any} */ { auth }) {
    const app = await getApp()
    return await app.suppressed.list()
  }

  /** Manually suppress an address from outbound delivery. */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN })
  static async POST(/** @type {any} */ { body }) {
    const address = String(body?.address || '').trim().toLowerCase()
    if (!address) return Respond.r400('address required')
    const reason = body?.reason === 'complaint' ? 'complaint' : 'bounce'
    const app = await getApp()
    await app.suppressed.suppress(address, reason)
    return { suppressed: address, reason }
  }
}
