import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

@RequestRoute("/snooze/tick")
@Public
export class SnoozeTickController {
  /** System-only due-snooze pass. */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  @PostRoute
  async post() {
    const app = await getApp()
    return await app.snoozeService.processDue()
  }
}
