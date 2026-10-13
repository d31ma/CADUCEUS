import { RouteRequest } from '../shared/http/RouteRequest.js'

@RequestRoute("/send/flush")
@Public
export class SendFlushController {
  /** @type {OutboundQueueService} */
  @Needs
  outboundQueueService

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
      const app = this
      const before = await app.outboundQueueService.status()
      const result = await app.outboundQueueService.processDue()
      return { flushed: result.delivered, ...result, before, queue: await app.outboundQueueService.status() }
    })
  }
}
