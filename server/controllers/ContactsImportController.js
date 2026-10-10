import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/contacts/import")
@SignedIn
export class ContactsImportController {
  /** @type {ContactsService} */
  @Needs
  contactsService

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (typeof body?.content !== 'string' || !body.content.trim()) return Respond.r400('vCard content is required')
      try { return await this.contactsService.importVCard(auth.email, body.content, body.groupIds || []) }
      catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid vCard') }
    })
  }
}
