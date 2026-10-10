import { RouteRequest } from '../shared/http/RouteRequest.js'

@RequestRoute("/notifications/vapid-public-key")
@SignedIn
export class NotificationsVapidPublicKeyController {
  /** @type {PushService} */
  @Needs
  pushService

  /**
 * GET /notifications/vapid-public-key
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = this
    return { publicKey: await app.pushService.getVapidPublicKey() }
    })
  }
}
