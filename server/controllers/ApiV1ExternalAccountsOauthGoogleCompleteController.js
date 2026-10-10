// @ts-check
import { Guards } from '../../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../../platform/accounts/bridge.js'

@RequestRoute("/api/v1/external-accounts/oauth/google/complete")
@Public
export class ApiV1ExternalAccountsOauthGoogleCompleteController {
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { context, body }) {
    try {
      return await ExternalAccounts.completeGoogleOAuth(bearerToken(context), body?.state || '', body?.code || '')
    } catch (error) {
      return accountFailure(error)
    }
  }
}
