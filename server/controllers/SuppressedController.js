import { RouteRequest } from '../middleware/RouteRequest.js'
import { Actions } from '../platform/auth/Auth.js'
import { getApp } from '../app.js'
import { Respond } from '../platform/http/Respond.js'

@RequestRoute("/suppressed")
@SignedIn
export class SuppressedController {
  /**
 * GET /suppressed
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.DOMAIN_ADMIN } }, async ({ auth }) => {
      const app = await getApp()
      return await app.suppressed.list()
    })
  }

  /** Manually suppress an address from outbound delivery. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.DOMAIN_ADMIN } }, async ({ body }) => {
      const address = String(body?.address || '').trim().toLowerCase()
      if (!address) return Respond.r400('address required')
      const reason = body?.reason === 'complaint' ? 'complaint' : 'bounce'
      const app = await getApp()
      await app.suppressed.suppress(address, reason)
      return { suppressed: address, reason }
    })
  }
}
