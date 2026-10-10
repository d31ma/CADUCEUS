import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

export class Handler {
  /**
   * POST /send/flush
   *
   * Run one delivery pass over the outbound queue: deliver due messages, retry
   * transient failures with backoff, and bounce permanent ones. Used for manual
   * recovery after a restart (the recurring worker does this automatically).
   *
   * @returns {Promise<Record<string, unknown>>}
   */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  static async POST() {
    const app = await getApp()
    const before = await app.outboundQueue.status()
    const result = await app.outboundQueue.processDue()
    return { flushed: result.delivered, ...result, before, queue: await app.outboundQueue.status() }
  }
}
