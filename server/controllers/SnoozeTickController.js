import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'

@RequestRoute("/snooze/tick")
@Public
export class SnoozeTickController {
  /** System-only due-snooze pass. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      const app = await getApp()
      return await app.snoozeService.processDue()
    })
  }
}
