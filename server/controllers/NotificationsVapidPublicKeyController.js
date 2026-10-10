import { RouteRequest } from '../middleware/RouteRequest.js'
import { getApp } from '../app.js'

@RequestRoute("/notifications/vapid-public-key")
@SignedIn
export class NotificationsVapidPublicKeyController {
  /**
 * GET /notifications/vapid-public-key
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = await getApp()
    return { publicKey: await app.push.getVapidPublicKey() }
    })
  }
}
