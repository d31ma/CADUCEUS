import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/contacts/groups")
@SignedIn
export class ContactsGroupsController {
  /** @type {ContactsService} */
  @Needs
  contactsService

  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
      return await this.contactsService.listGroups(auth.email)
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      try { return await this.contactsService.createGroup(auth.email, body?.name) }
      catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
    })
  }
}
