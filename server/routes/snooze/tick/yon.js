import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

export class Handler {
  /** System-only due-snooze pass. */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  static async POST() {
    const app = await getApp()
    return await app.snoozeService.processDue()
  }
}
