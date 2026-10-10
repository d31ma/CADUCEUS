import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

@RequestRoute("/reminders/tick")
@Public
export class RemindersTickController {
  /** System-only pass over due follow-up reminders for every mailbox. */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  @PostRoute
  async post() {
    const app = await getApp()
    return await app.reminders.processDueAll()
  }
}
