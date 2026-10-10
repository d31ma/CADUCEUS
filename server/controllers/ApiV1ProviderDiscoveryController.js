// @ts-check
import { Guards } from '../../../../middleware/guards.js'
import { Respond } from '../../../../platform/http/respond.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../platform/accounts/bridge.js'

@RequestRoute("/api/v1/provider-discovery")
@Public
export class ApiV1ProviderDiscoveryController {
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { context, body }) {
    const address = typeof body?.address === 'string' ? body.address.trim() : ''
    if (!address) return Respond.r400('address is required')
    try {
      return await ExternalAccounts.discover(bearerToken(context), address)
    } catch (error) {
      return accountFailure(error)
    }
  }
}
