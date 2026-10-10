// @ts-check
import { Guards } from '../../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../../platform/accounts/bridge.js'

@RequestRoute("/api/v1/external-accounts/oauth/microsoft/begin")
@Public
export class ApiV1ExternalAccountsOauthMicrosoftBeginController {
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { context, body }) {
    try {
      return await ExternalAccounts.beginMicrosoftOAuth(bearerToken(context), body || {})
    } catch (error) {
      return accountFailure(error)
    }
  }
}
