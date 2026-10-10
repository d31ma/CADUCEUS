// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'

@RequestRoute("/auth/token/refresh")
@Public
export class AuthTokenRefreshController {
  /**
   * POST /auth/token/refresh — rotate one refresh token.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  @PostRoute
  async post(/** @type {any} */ { body, context }) {
    const refreshToken = String(body?.refreshToken ?? '').trim()
    if (!refreshToken) return Respond.r400('refreshToken is required')
    const clientId = String(body?.clientId ?? '').trim()
    if (!clientId) return Respond.r400('clientId is required')

    try {
      return await Auth.refresh(refreshToken, clientId)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
