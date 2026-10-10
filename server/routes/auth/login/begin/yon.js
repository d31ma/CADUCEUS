// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'
import { getApp } from '../../../../app.js'

export class Handler {
  /**
   * POST /auth/login/begin — start one authentication transaction.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  static async POST(/** @type {any} */ { body, context }) {
    const address = String(body?.address ?? '').trim()
    if (!address) return Respond.r400('address is required')

    const app = await getApp()
    const limit = await app.rateLimit.check(
      ['login-begin', address, context?.ipAddress ?? 'unknown'], 20, 15 * 60 * 1000,
    )
    if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

    try {
      return await Auth.loginBegin(address)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
