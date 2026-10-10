// @ts-check
import { Guards } from '../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../platform/accounts/bridge.js'

@RequestRoute("/api/v1/external-accounts")
@Public
export class ApiV1ExternalAccountsController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { context }) {
    try {
      return { accounts: await ExternalAccounts.list(bearerToken(context)) }
    } catch (error) {
      return accountFailure(error)
    }
  }

  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { context, body }) {
    try {
      // Spreading happens inside ExternalAccounts.create and the trusted token
      // is written last, so a body token can never select another owner.
      return await ExternalAccounts.create(bearerToken(context), body || {})
    } catch (error) {
      return accountFailure(error)
    }
  }
}
