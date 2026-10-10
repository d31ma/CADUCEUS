import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

export class Handler {
  /**
 * GET /notifications/vapid-public-key
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  static async GET(/** @type {any} */ { auth }) {
  const app = await getApp()
  return { publicKey: await app.push.getVapidPublicKey() }
}
}
