// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/provider-catalog")
@SignedIn
export class ApiV1ProviderCatalogController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      try {
        return { providers: await ExternalAccounts.providers(bearerToken(context)) }
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
