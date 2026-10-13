import { RouteRequest } from '../shared/http/RouteRequest.js'

@RequestRoute("/reminders/tick")
@Public
export class RemindersTickController {
  /** @type {FollowUpReminderService} */
  @Needs
  followUpReminderService

  /** System-only pass over due follow-up reminders for every mailbox. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      const app = this
      return await app.followUpReminderService.processDueAll()
    })
  }
}
