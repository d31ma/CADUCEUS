import { RouteRequest } from '../middleware/RouteRequest.js'
// @ts-check
import { Respond } from '../platform/http/Respond.js'
import { Auth } from '../platform/auth/Auth.js'
import { AuthGuard } from '../middleware/AuthGuard.js'

@RequestRoute("/auth/session")
@Public
export class AuthSessionController {
  /**
   * GET /auth/session — the caller's own resolved principal. A client uses it
   * to confirm a stored token still speaks for a live session.
   *
   * @param {any} request
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, {  }, async ({ context }) => {
      const result = await AuthGuard.requireAuth(context ?? {})
      return result.ok ? result.principal : result.response
    })
  }

  /**
   * DELETE /auth/session — end the caller's own session.
   *
   * @param {any} request
   */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, {  }, async ({ context }) => {
      const token = context?.bearer?.token
      if (!token) return Respond.r401('Authentication required')
      try {
        return await Auth.revokeSession(token, 'signed out')
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
