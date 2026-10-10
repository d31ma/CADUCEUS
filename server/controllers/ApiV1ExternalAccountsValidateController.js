// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/validate")
@SignedIn
export class ApiV1ExternalAccountsValidateController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        return await ExternalAccounts.validate(ExternalAccounts.bearerToken(context), body || {})
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }
}
