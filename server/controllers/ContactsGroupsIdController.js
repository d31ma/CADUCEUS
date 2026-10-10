import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/contacts/groups/{id}")
@SignedIn
export class ContactsGroupsIdController {
  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth }) => {
      try {
        return await (await getApp()).contacts.updateGroup(String(paths?.id || ''), auth.email, body?.name) ?? Respond.r404('Contact group not found')
      } catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth }) => {
      if (!await (await getApp()).contacts.deleteGroup(String(paths?.id || ''), auth.email)) return Respond.r404('Contact group not found')
      return { deleted: true }
    })
  }
}
