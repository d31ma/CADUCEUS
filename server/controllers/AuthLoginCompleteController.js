// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'

@RequestRoute("/auth/login/complete")
@Public
export class AuthLoginCompleteController {
  /**
   * POST /auth/login/complete — issue the session and its token set.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  @PostRoute
  async post(/** @type {any} */ { body, context }) {
    const transactionId = String(body?.transactionId ?? '').trim()
    if (!transactionId) return Respond.r400('transactionId is required')

    try {
      return await Auth.completeLogin(transactionId)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
