import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/contacts/suggestions")
@SignedIn
export class ContactsSuggestionsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query, context, auth }) => {
      const q = String((query || context?.query || {}).q || '').slice(0, 200)
      let providerContacts = []
      try { providerContacts = (await ExternalAccounts.contacts(ExternalAccounts.bearerToken(context)))?.contacts || [] } catch { /* keep local completion available */ }
      return await (await getApp()).contacts.suggestions(auth.email, q, providerContacts)
    })
  }
}
