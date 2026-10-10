import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/credentials/password")
@Public
export class AuthCredentialsPasswordController {
  /**
   * PUT /auth/credentials/password — set the caller's password.
   *
   * Changing a credential requires a freshly satisfied authentication
   * transaction, proven by the boundary against this same principal.
   *
   * @param {any} request
   */
  @PutRoute
  async put(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
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
    })
  }
}
