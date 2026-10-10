// @ts-check
import { RouteRequest } from '../middleware/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/validate")
@SignedIn
export class ApiV1ExternalAccountsValidateController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        return await ExternalAccounts.validate(bearerToken(context), body || {})
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
