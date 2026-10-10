import { RouteRequest } from '../shared/http/RouteRequest.js'
// @ts-check
import { Auth } from '../shared/auth/Auth.js'
import { AuthGuard } from '../shared/http/AuthGuard.js'

@RequestRoute("/auth/jwks")
@Public
export class AuthJwksController {
  /**
   * GET /auth/jwks — the engine's public signing keys. Published so a client
   * or an external verifier can check a CADUCEUS token without asking this
   * service about it. It carries no private material.
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, {  }, async () => {
      try {
        return await Auth.jwks()
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
