import { Guards } from '../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { context }) {
    try {
      return (await ExternalAccounts.contactBooks(bearerToken(context)))?.addressBooks || []
    } catch (error) {
      return accountFailure(error)
    }
  }
}
