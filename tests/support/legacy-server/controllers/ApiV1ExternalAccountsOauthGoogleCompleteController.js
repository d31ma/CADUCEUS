// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/oauth/google/complete")
@SignedIn
export class ApiV1ExternalAccountsOauthGoogleCompleteController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        return await ExternalAccounts.completeGoogleOAuth(ExternalAccounts.bearerToken(context), body?.state || '', body?.code || '')
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }
}
