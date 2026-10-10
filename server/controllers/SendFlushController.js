import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'

@RequestRoute("/send/flush")
@Public
export class SendFlushController {
  /**
   * POST /send/flush
   *
   * Run one delivery pass over the outbound queue: deliver due messages, retry
   * transient failures with backoff, and bounce permanent ones. Used for manual
   * recovery after a restart (the recurring worker does this automatically).
   *
   * @returns {Promise<Record<string, unknown>>}
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      const app = await getApp()
      const before = await app.outboundQueue.status()
      const result = await app.outboundQueue.processDue()
      return { flushed: result.delivered, ...result, before, queue: await app.outboundQueue.status() }
    })
  }
}
