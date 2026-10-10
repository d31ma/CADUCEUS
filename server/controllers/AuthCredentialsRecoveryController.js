// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'

@RequestRoute("/auth/credentials/recovery")
@Public
export class AuthCredentialsRecoveryController {
  /**
   * POST /auth/credentials/recovery — replace the caller's recovery codes.
   *
   * Changing a credential requires a freshly satisfied authentication
   * transaction, proven by the boundary against this same principal.
   *
   * @param {any} request
   */
  @PostRoute
  async post(/** @type {any} */ { body, context }) {
    const token = context?.bearer?.token
    if (!token) return Respond.r401('Authentication required')
    const stepUpTransactionId = String(body?.stepUpTransactionId ?? '').trim()
    if (!stepUpTransactionId) return Respond.r400('stepUpTransactionId is required')
    try {
      return await Auth.recoveryCodesIssue(token, stepUpTransactionId)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
