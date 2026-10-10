// @ts-check
import { Guards } from '../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../platform/accounts/bridge.js'

@RequestRoute("/api/v1/provider-catalog")
@Public
export class ApiV1ProviderCatalogController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { context }) {
    try {
      return { providers: await ExternalAccounts.providers(bearerToken(context)) }
    } catch (error) {
      return accountFailure(error)
    }
  }
}
