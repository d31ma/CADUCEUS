// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/provider-discovery")
@SignedIn
export class ApiV1ProviderDiscoveryController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      const address = typeof body?.address === 'string' ? body.address.trim() : ''
      if (!address) return Respond.r400('address is required')
      try {
        return await ExternalAccounts.discover(bearerToken(context), address)
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
