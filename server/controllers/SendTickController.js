import { RouteRequest } from '../middleware/RouteRequest.js'
import { getApp } from '../app.js'

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
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      const app = await getApp()
      const result = await app.outboundQueue.processDue()
      return { ...result, queue: await app.outboundQueue.status() }
    })
  }
}
