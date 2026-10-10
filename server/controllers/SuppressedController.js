import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/suppressed")
@SignedIn
export class SuppressedController {
  /** @type {SuppressedHelper} */
  @Needs
  suppressedHelper

  /**
 * GET /suppressed
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.DOMAIN_ADMIN } }, async ({ auth }) => {
      const app = this
      return await app.suppressedHelper.list()
    })
  }

  /** Manually suppress an address from outbound delivery. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.DOMAIN_ADMIN } }, async ({ body }) => {
      const address = String(body?.address || '').trim().toLowerCase()
      if (!address) return Respond.r400('address required')
      const reason = body?.reason === 'complaint' ? 'complaint' : 'bounce'
      const app = this
      await app.suppressedHelper.suppress(address, reason)
      return { suppressed: address, reason }
    })
  }
}
