import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { SnoozeError } from '../features/mailbox/services/SnoozeService.js'

@RequestRoute("/snooze/{id}")
@SignedIn
export class SnoozeIdController {
  /** @param {{paths?:{id?:string},auth:any}} params */
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth }) => {
      if (!paths?.id) return Respond.r404('Snooze not found')
      const app = await getApp()
      try {
        return await app.snoozeService.undo(paths.id, auth.email)
      } catch (error) {
        if (error instanceof SnoozeError && error.status === 404) return Respond.r404(error.message)
        throw error
      }
    })
  }
}
