import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/contacts/groups")
@SignedIn
export class ContactsGroupsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
      return await (await getApp()).contacts.listGroups(auth.email)
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      try { return await (await getApp()).contacts.createGroup(auth.email, body?.name) }
      catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
    })
  }
}
