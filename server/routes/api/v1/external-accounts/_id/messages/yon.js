// @ts-check
import { Guards } from '../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { context, paths, query }) {
    const limit = Math.max(1, Math.min(500, Number(query?.limit) || 50))
    try { return await ExternalAccounts.messages(bearerToken(context), String(paths?.id || ''), String(query?.folder || ''), limit) }
    catch (error) { return accountFailure(error) }
  }
}
