// @ts-check
import { Auth } from '../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../middleware/require-auth.js'

export class Handler {
  /**
   * GET /auth/jwks — the engine's public signing keys. Published so a client
   * or an external verifier can check a CADUCEUS token without asking this
   * service about it. It carries no private material.
   */
  static async GET() {
    try {
      return await Auth.jwks()
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
