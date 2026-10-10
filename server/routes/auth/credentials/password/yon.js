// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'

export class Handler {
  /**
   * PUT /auth/credentials/password — set the caller's password.
   *
   * Changing a credential requires a freshly satisfied authentication
   * transaction, proven by the boundary against this same principal.
   *
   * @param {any} request
   */
  static async PUT(/** @type {any} */ { body, context }) {
    const token = context?.bearer?.token
    if (!token) return Respond.r401('Authentication required')
    const stepUpTransactionId = String(body?.stepUpTransactionId ?? '').trim()
    if (!stepUpTransactionId) return Respond.r400('stepUpTransactionId is required')
    const password = String(body?.password ?? '').trim()
    if (!password) return Respond.r400('password is required')
    try {
      return await Auth.setPassword(token, stepUpTransactionId, password)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
