// @ts-check
import { Auth } from '../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../middleware/require-auth.js'

@RequestRoute("/auth/jwks")
@Public
export class AuthJwksController {
  /**
   * GET /auth/jwks — the engine's public signing keys. Published so a client
   * or an external verifier can check a CADUCEUS token without asking this
   * service about it. It carries no private material.
   */
  @GetRoute
  async get() {
    try {
      return await Auth.jwks()
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
