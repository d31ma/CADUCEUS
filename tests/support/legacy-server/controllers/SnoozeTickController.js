import { RouteRequest } from '../shared/http/RouteRequest.js'

@RequestRoute("/snooze/tick")
@Public
export class SnoozeTickController {
  /** @type {SnoozeService} */
  @Needs
  snoozeService

  /** System-only due-snooze pass. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      const app = this
      return await app.snoozeService.processDue()
    })
  }
}
