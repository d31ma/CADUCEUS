// @ts-check
import { Guards } from '../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../platform/accounts/bridge.js'

@RequestRoute("/api/v1/external-accounts/{id}/diagnostics")
@Public
export class ApiV1ExternalAccountsIdDiagnosticsController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { context, paths }) {
    try {
      return await ExternalAccounts.diagnostics(bearerToken(context), String(paths?.id || ''))
    } catch (error) {
      return accountFailure(error)
    }
  }
}
