import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

@RequestRoute("/notifications/vapid-public-key")
@Public
export class NotificationsVapidPublicKeyController {
  /**
 * GET /notifications/vapid-public-key
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { auth }) {
  const app = await getApp()
  return { publicKey: await app.push.getVapidPublicKey() }
}
}
