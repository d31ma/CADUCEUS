import { Respond } from '../../../platform/http/respond.js'
import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

export class Handler {
  /**
 * POST /inbound/webhook
 *
 * Canonical signed inbound endpoint. The body must be JSON and the request
 * must carry an `X-Caduceus-Signature` HMAC-SHA256 computed over the exact
 * JSON body using `INBOUND_WEBHOOK_SECRET`. The signature is verified by the
 * `@Guards.webhookSignature` guard before the body runs.
 *
 * @param {object} params
 * @param {{ recipient: string, sender: string, subject?: string, body?: string, messageId?: string }} params.body
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.webhookSignature('INBOUND_WEBHOOK_SECRET')
  static async POST(/** @type {any} */ { body }) {
  if (!body?.recipient || !body.sender) return Respond.r400('recipient and sender required')

  const app = await getApp()
  const result = await app.inboundDelivery.deliver(body)
  return result.body
}
}
