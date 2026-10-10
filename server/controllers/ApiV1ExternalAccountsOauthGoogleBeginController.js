// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/oauth/google/begin")
@SignedIn
export class ApiV1ExternalAccountsOauthGoogleBeginController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        return await ExternalAccounts.beginGoogleOAuth(bearerToken(context), body || {})
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
