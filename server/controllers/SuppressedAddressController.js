import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/suppressed/{address}")
@SignedIn
export class SuppressedAddressController {
  /**
 * DELETE /suppressed/_address
 * @param {object} params
 * @param {{ address: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(@FromPath address, request) {
    return RouteRequest.handle(request, { paths: { address }, guard: { action: Auth.Actions.DOMAIN_ADMIN } }, async ({ paths, auth }) => {
    const address = paths?.address
    if (!address) return Respond.r400('address required')

    const app = await getApp()
    await app.suppressed.delete(address)
    return { removed: address }
    })
  }
}
