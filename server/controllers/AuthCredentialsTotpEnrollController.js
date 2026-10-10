import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/credentials/totp/enroll")
@Public
export class AuthCredentialsTotpEnrollController {
  /**
   * POST /auth/credentials/totp/enroll — issue a TOTP secret, once.
   *
   * Changing a credential requires a freshly satisfied authentication
   * transaction, proven by the boundary against this same principal.
   *
   * @param {any} request
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
      const token = context?.bearer?.token
      if (!token) return Respond.r401('Authentication required')
      const stepUpTransactionId = String(body?.stepUpTransactionId ?? '').trim()
      if (!stepUpTransactionId) return Respond.r400('stepUpTransactionId is required')
      try {
        return await Auth.totpEnroll(token, stepUpTransactionId)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
