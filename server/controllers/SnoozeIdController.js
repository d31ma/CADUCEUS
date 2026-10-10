import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { SnoozeError } from '../shared/errors/SnoozeError.js'

@RequestRoute("/snooze/{id}")
@SignedIn
export class SnoozeIdController {
  /** @type {SnoozeService} */
  @Needs
  snoozeService

  /** @param {{paths?:{id?:string},auth:any}} params */
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth }) => {
      if (!paths?.id) return Respond.r404('Snooze not found')
      const app = this
      try {
        return await app.snoozeService.undo(paths.id, auth.email)
      } catch (error) {
        if (error instanceof SnoozeError && error.status === 404) return Respond.r404(error.message)
        throw error
      }
    })
  }
}
