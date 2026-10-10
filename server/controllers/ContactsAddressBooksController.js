import { Guards } from '../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../platform/accounts/bridge.js'

@RequestRoute("/contacts/address-books")
@Public
export class ContactsAddressBooksController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { context }) {
    try {
      return (await ExternalAccounts.contactBooks(bearerToken(context)))?.addressBooks || []
    } catch (error) {
      return accountFailure(error)
    }
  }
}
