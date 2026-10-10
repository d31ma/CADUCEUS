// @ts-check
import { RouteRequest } from '../middleware/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts")
@SignedIn
export class ApiV1ExternalAccountsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      try {
        return { accounts: await ExternalAccounts.list(bearerToken(context)) }
      } catch (error) {
        return accountFailure(error)
      }
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        // Spreading happens inside ExternalAccounts.create and the trusted token
        // is written last, so a body token can never select another owner.
        return await ExternalAccounts.create(bearerToken(context), body || {})
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
