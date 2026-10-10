import { RouteRequest } from '../middleware/RouteRequest.js'
// @ts-check
import { Respond } from '../platform/http/Respond.js'
import { Auth } from '../platform/auth/Auth.js'
import { AuthGuard } from '../middleware/AuthGuard.js'

@RequestRoute("/auth/credentials/passkey")
@Public
export class AuthCredentialsPasskeyController {
  /**
   * GET /auth/credentials/passkey — the caller's own registered passkeys.
   *
   * @param {any} request
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, {  }, async ({ context }) => {
      const token = context?.bearer?.token
      if (!token) return Respond.r401('Authentication required')
      try {
        return await Auth.passkeyList(token)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }

  /**
   * DELETE /auth/credentials/passkey — unregister one of the caller's own
   * passkeys. The boundary refuses a credential that belongs to anyone else.
   *
   * @param {any} request
   */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, {  }, async ({ body, context }) => {
      const token = context?.bearer?.token
      if (!token) return Respond.r401('Authentication required')
      const stepUpTransactionId = String(body?.stepUpTransactionId ?? '').trim()
      const credentialId = String(body?.credentialId ?? '').trim()
      if (!stepUpTransactionId) return Respond.r400('stepUpTransactionId is required')
      if (!credentialId) return Respond.r400('credentialId is required')
      try {
        return await Auth.passkeyRemove(token, stepUpTransactionId, credentialId)
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }
}
