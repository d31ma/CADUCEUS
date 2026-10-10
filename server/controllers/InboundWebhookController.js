import { Respond } from '../platform/http/Respond.js'
import { RouteRequest } from '../middleware/RouteRequest.js'
import { getApp } from '../app.js'

@RequestRoute("/inbound/webhook")
@Public
export class InboundWebhookController {
  /**
 * POST /inbound/webhook
 *
 * Canonical signed inbound endpoint. The body must be JSON and the request
 * must carry an `X-Caduceus-Signature` HMAC-SHA256 computed over the exact
 * JSON body using `INBOUND_WEBHOOK_SECRET`. The signature is verified by the
 * `signature` route check before the body runs.
 *
 * @param {object} params
 * @param {{ recipient: string, sender: string, subject?: string, body?: string, messageId?: string }} params.body
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async ({ body }) => {
    if (!body?.recipient || !body.sender) return Respond.r400('recipient and sender required')

    const app = await getApp()
    const result = await app.inboundDelivery.deliver(body)
    return result.body
    })
  }
}
