import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/login/begin")
@Public
export class AuthLoginBeginController {
  /** @type {RateLimitService} */
  @Needs
  rateLimitService

  /**
   * POST /auth/login/begin — start one authentication transaction.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
      const address = String(body?.address ?? '').trim()
      if (!address) return Respond.r400('address is required')

      const app = this
      const limit = await app.rateLimitService.check(
        ['login-begin', address, context?.ipAddress ?? 'unknown'], 20, 15 * 60 * 1000,
      )
      if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

      try {
        return await Auth.loginBegin(address)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
