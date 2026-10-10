import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/contacts/groups/{id}")
@SignedIn
export class ContactsGroupsIdController {
  /** @type {ContactsService} */
  @Needs
  contactsService

  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth }) => {
      try {
        return await this.contactsService.updateGroup(String(paths?.id || ''), auth.email, body?.name) ?? Respond.r404('Contact group not found')
      } catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth }) => {
      if (!await this.contactsService.deleteGroup(String(paths?.id || ''), auth.email)) return Respond.r404('Contact group not found')
      return { deleted: true }
    })
  }
}
