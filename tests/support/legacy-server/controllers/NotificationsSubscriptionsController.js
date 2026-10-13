import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { PushSubscriptionsRecords } from '../shared/notifications/PushSubscriptionsRecords.js'

@RequestRoute("/notifications/subscriptions")
@SignedIn
export class NotificationsSubscriptionsController {
  /** @type {PushSubscriptionsHelper} */
  @Needs
  pushSubscriptionsHelper

  /**
 * POST /notifications/subscriptions
 * @param {object} params
 * @param {{ endpoint: string, keys: { p256dh: string, auth: string } }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @param {Record<string, string>} params.headers
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, headers, auth }) => {
    const subscription = body
    if (!subscription?.endpoint || !subscription.keys?.p256dh || !subscription.keys.auth) {
      return Respond.r400('endpoint, p256dh, and auth are required')
    }

    const app = this
    const record = await app.pushSubscriptionsHelper.upsert(/** @type {any} */ ({
      userEmail: auth.email,
      endpoint: subscription.endpoint,
      keys: { p256dh: subscription.keys.p256dh, auth: subscription.keys.auth },
      userAgent: headers?.['user-agent'] ?? headers?.['User-Agent'],
    }))
    const { keys, ...safe } = record
    return safe
    })
  }

  /**
 * DELETE /notifications/subscriptions
 * @param {object} params
 * @param {{ endpoint: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
    if (!body?.endpoint) return Respond.r400('endpoint required')

    const app = this
    const [docId, subscription] = await app.pushSubscriptionsHelper.findById(PushSubscriptionsRecords.subscriptionId(body.endpoint))
    if (!docId || !subscription || subscription.userEmail !== auth.email.toLowerCase()) {
      return Respond.r404('Subscription not found')
    }
    await app.pushSubscriptionsHelper.deleteDoc(docId)
    return { deleted: subscription.id }
    })
  }

  /**
 * GET /notifications/subscriptions
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = this
    const subscriptions = await app.pushSubscriptionsHelper.listForUser(auth.email)
    // Strip secrets (`keys`) and the internal docId from the wire payload.
    return subscriptions.map(({ docId, keys, ...rest }) => rest)
    })
  }
}
