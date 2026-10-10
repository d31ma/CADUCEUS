import { RouteRequest } from '../middleware/RouteRequest.js'
// @ts-check
import { Respond } from '../platform/http/Respond.js'
import { Auth } from '../platform/auth/Auth.js'
import { AuthGuard } from '../middleware/AuthGuard.js'

@RequestRoute("/auth/logout")
@Public
export class AuthLogoutController {
  /**
   * POST /auth/logout — end the session an ID token speaks for.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
      const idToken = String(body?.idToken ?? '').trim()
      if (!idToken) return Respond.r400('idToken is required')

      try {
        return await Auth.logout(idToken)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
