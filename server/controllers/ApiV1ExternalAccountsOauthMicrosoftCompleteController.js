// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/oauth/microsoft/complete")
@SignedIn
export class ApiV1ExternalAccountsOauthMicrosoftCompleteController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        return await ExternalAccounts.completeMicrosoftOAuth(bearerToken(context), body?.state || '', body?.code || '')
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
