import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'

@RequestRoute("/reminders/tick")
@Public
export class RemindersTickController {
  /** System-only pass over due follow-up reminders for every mailbox. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      const app = await getApp()
      return await app.reminders.processDueAll()
    })
  }
}
