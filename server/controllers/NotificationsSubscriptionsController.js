import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { PushSubscriptionsRepository } from '../../../features/notifications/repositories/push.js'

@RequestRoute("/notifications/subscriptions")
@Public
export class NotificationsSubscriptionsController {
  /**
 * POST /notifications/subscriptions
 * @param {object} params
 * @param {{ endpoint: string, keys: { p256dh: string, auth: string } }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @param {Record<string, string>} params.headers
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, headers, auth }) {
  const subscription = body
  if (!subscription?.endpoint || !subscription.keys?.p256dh || !subscription.keys.auth) {
    return Respond.r400('endpoint, p256dh, and auth are required')
  }

  const app = await getApp()
  const record = await app.pushSubscriptions.upsert(/** @type {any} */ ({
    userEmail: auth.email,
    endpoint: subscription.endpoint,
    keys: { p256dh: subscription.keys.p256dh, auth: subscription.keys.auth },
    userAgent: headers?.['user-agent'] ?? headers?.['User-Agent'],
  }))
  const { keys, ...safe } = record
  return safe
}

  /**
 * DELETE /notifications/subscriptions
 * @param {object} params
 * @param {{ endpoint: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  @DeleteRoute
  async delete(/** @type {any} */ { body, auth }) {
  if (!body?.endpoint) return Respond.r400('endpoint required')

  const app = await getApp()
  const [docId, subscription] = await app.pushSubscriptions.findById(PushSubscriptionsRepository.subscriptionId(body.endpoint))
  if (!docId || !subscription || subscription.userEmail !== auth.email.toLowerCase()) {
    return Respond.r404('Subscription not found')
  }
  await app.pushSubscriptions.deleteDoc(docId)
  return { deleted: subscription.id }
}

  /**
 * GET /notifications/subscriptions
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { auth }) {
  const app = await getApp()
  const subscriptions = await app.pushSubscriptions.listForUser(auth.email)
  // Strip secrets (`keys`) and the internal docId from the wire payload.
  return subscriptions.map(({ docId, keys, ...rest }) => rest)
}
}
