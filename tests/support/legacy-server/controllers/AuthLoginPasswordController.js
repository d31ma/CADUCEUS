import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/login/password")
@Public
export class AuthLoginPasswordController {
  /** @type {RateLimitService} */
  @Needs
  rateLimitService

  /**
   * POST /auth/login/password — supply the password factor.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
      const transactionId = String(body?.transactionId ?? '').trim()
      if (!transactionId) return Respond.r400('transactionId is required')
      const password = String(body?.password ?? '').trim()
      if (!password) return Respond.r400('password is required')

      const app = this
      const limit = await app.rateLimitService.check(
        ['login-password', transactionId, context?.ipAddress ?? 'unknown'], 12, 15 * 60 * 1000,
      )
      if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

      try {
        return await Auth.verifyPassword(transactionId, password)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
