// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'
import { getApp } from '../../../../app.js'

export class Handler {
  /**
   * POST /auth/login/password — supply the password factor.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  static async POST(/** @type {any} */ { body, context }) {
    const transactionId = String(body?.transactionId ?? '').trim()
    if (!transactionId) return Respond.r400('transactionId is required')
    const password = String(body?.password ?? '').trim()
    if (!password) return Respond.r400('password is required')

    const app = await getApp()
    const limit = await app.rateLimit.check(
      ['login-password', transactionId, context?.ipAddress ?? 'unknown'], 12, 15 * 60 * 1000,
    )
    if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

    try {
      return await Auth.verifyPassword(transactionId, password)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
