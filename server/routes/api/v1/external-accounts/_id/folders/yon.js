// @ts-check
import { Guards } from '../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { context, paths }) {
    try { return await ExternalAccounts.folders(bearerToken(context), String(paths?.id || '')) }
    catch (error) { return accountFailure(error) }
  }
}
