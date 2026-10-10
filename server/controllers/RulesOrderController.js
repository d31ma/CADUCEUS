import { RouteRequest } from '../middleware/RouteRequest.js'
import { Actions } from '../platform/auth/Auth.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/rules/order")
@SignedIn
export class RulesOrderController {
  @PutRoute
  async put(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
      const ids = body?.ids
      if (!Array.isArray(ids) || ids.length > 100 || ids.some(id => typeof id !== 'string') || new Set(ids).size !== ids.length) {
        return Respond.r400('ids must be a unique list of rule IDs')
      }
      const app = await getApp()
      if (!await app.rules.reorder(auth.email, ids)) return Respond.r400('ids must include every owned rule exactly once')
      return { ordered: ids.length }
    })
  }
}
