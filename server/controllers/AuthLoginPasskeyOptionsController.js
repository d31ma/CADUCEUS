import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/login/passkey/options")
@Public
export class AuthLoginPasskeyOptionsController {
  /**
   * POST /auth/login/passkey/options — challenge for an in-flight login.
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

      try {
        return await Auth.passkeyOptions(transactionId)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
