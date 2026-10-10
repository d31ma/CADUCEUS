import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

export class Handler {
  /** System-only pass over due follow-up reminders for every mailbox. */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  static async POST() {
    const app = await getApp()
    return await app.reminders.processDueAll()
  }
}
