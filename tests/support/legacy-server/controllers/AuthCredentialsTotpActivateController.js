import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/credentials/totp/activate")
@Public
export class AuthCredentialsTotpActivateController {
  /**
   * POST /auth/credentials/totp/activate — prove a code and enable TOTP.
   *
   * @param {any} request
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
      const token = context?.bearer?.token
      if (!token) return Respond.r401('Authentication required')
      const code = String(body?.code ?? '').trim()
      if (!code) return Respond.r400('code is required')
      try {
        return await Auth.totpActivate(token, code)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
