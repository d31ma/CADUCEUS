// @ts-check
import { Guards } from '../../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async POST(/** @type {any} */ { context, body }) {
    try {
      return await ExternalAccounts.completeMicrosoftOAuth(bearerToken(context), body?.state || '', body?.code || '')
    } catch (error) {
      return accountFailure(error)
    }
  }
}
