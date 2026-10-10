import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/contacts/address-books")
@SignedIn
export class ContactsAddressBooksController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      try {
        return (await ExternalAccounts.contactBooks(bearerToken(context)))?.addressBooks || []
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
