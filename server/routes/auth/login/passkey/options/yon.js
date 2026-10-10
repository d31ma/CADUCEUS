// @ts-check
import { Respond } from '../../../../../platform/http/respond.js'
import { Auth } from '../../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../../middleware/require-auth.js'

export class Handler {
  /**
   * POST /auth/login/passkey/options — challenge for an in-flight login.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  static async POST(/** @type {any} */ { body, context }) {
    const transactionId = String(body?.transactionId ?? '').trim()
    if (!transactionId) return Respond.r400('transactionId is required')

    try {
      return await Auth.passkeyOptions(transactionId)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
