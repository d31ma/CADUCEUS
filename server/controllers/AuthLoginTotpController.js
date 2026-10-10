import { RouteRequest } from '../middleware/RouteRequest.js'
// @ts-check
import { Respond } from '../platform/http/Respond.js'
import { Auth } from '../platform/auth/Auth.js'
import { AuthGuard } from '../middleware/AuthGuard.js'
import { getApp } from '../app.js'

@RequestRoute("/auth/login/totp")
@Public
export class AuthLoginTotpController {
  /**
   * POST /auth/login/totp — supply a time-based code.
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

      const app = await getApp()
      const limit = await app.rateLimit.check(
        ['login-totp', transactionId, context?.ipAddress ?? 'unknown'], 10, 5 * 60 * 1000,
      )
      if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

      try {
        return await Auth.verifyTotp(transactionId, code)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
