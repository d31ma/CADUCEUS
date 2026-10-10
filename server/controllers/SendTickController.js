import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

@RequestRoute("/send/tick")
@Public
export class SendTickController {
  /**
   * POST /send/tick
   *
   * System-triggered delivery pass over the outbound queue (deliver due
   * messages, retry transient failures with backoff, bounce permanent ones).
   * Called on an interval by the delivery worker in `scripts/entry.mjs`, so it
   * uses the same HMAC signature as the inbound webhook rather than a user
   * session — it must not be reachable with a normal bearer token.
   *
   * @returns {Promise<Record<string, unknown>>}
   */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  @PostRoute
  async post() {
    const app = await getApp()
    const result = await app.outboundQueue.processDue()
    return { ...result, queue: await app.outboundQueue.status() }
  }
}
