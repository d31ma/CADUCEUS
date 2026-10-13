// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts")
@SignedIn
export class ApiV1ExternalAccountsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      try {
        return { accounts: await ExternalAccounts.list(ExternalAccounts.bearerToken(context)) }
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, body }) => {
      try {
        // Spreading happens inside ExternalAccounts.create and the trusted token
        // is written last, so a body token can never select another owner.
        return await ExternalAccounts.create(ExternalAccounts.bearerToken(context), body || {})
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }
}
