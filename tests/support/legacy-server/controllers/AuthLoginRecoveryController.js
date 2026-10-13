import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/login/recovery")
@Public
export class AuthLoginRecoveryController {
  /** @type {RateLimitService} */
  @Needs
  rateLimitService

  /**
   * POST /auth/login/recovery — supply a single-use recovery code.
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
      const code = String(body?.code ?? '').trim()
      if (!code) return Respond.r400('code is required')

      const app = this
      const limit = await app.rateLimitService.check(
        ['login-recovery', transactionId, context?.ipAddress ?? 'unknown'], 10, 15 * 60 * 1000,
      )
      if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

      try {
        return await Auth.verifyRecoveryCode(transactionId, code)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
