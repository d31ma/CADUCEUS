import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'
import { ExternalAccounts, bearerToken } from '../../../platform/accounts/bridge.js'

@RequestRoute("/contacts/suggestions")
@Public
export class ContactsSuggestionsController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { query, context, auth }) {
    const q = String((query || context?.query || {}).q || '').slice(0, 200)
    let providerContacts = []
    try { providerContacts = (await ExternalAccounts.contacts(bearerToken(context)))?.contacts || [] } catch { /* keep local completion available */ }
    return await (await getApp()).contacts.suggestions(auth.email, q, providerContacts)
  }
}
