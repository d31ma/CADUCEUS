import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/contacts/import")
@SignedIn
export class ContactsImportController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (typeof body?.content !== 'string' || !body.content.trim()) return Respond.r400('vCard content is required')
      try { return await (await getApp()).contacts.importVCard(auth.email, body.content, body.groupIds || []) }
      catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid vCard') }
    })
  }
}
