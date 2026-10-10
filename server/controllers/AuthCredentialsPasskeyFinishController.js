// @ts-check
import { Respond } from '../../../../../platform/http/respond.js'
import { Auth } from '../../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../../middleware/require-auth.js'

@RequestRoute("/auth/credentials/passkey/finish")
@Public
export class AuthCredentialsPasskeyFinishController {
  /**
   * POST /auth/credentials/passkey/finish — store a verified passkey.
   *
   * @param {any} request
   */
  @PostRoute
  async post(/** @type {any} */ { body, context }) {
    const token = context?.bearer?.token
    if (!token) return Respond.r401('Authentication required')
    const attestationObject = String(body?.attestationObject ?? '').trim()
    if (!attestationObject) return Respond.r400('attestationObject is required')
    const clientDataJSON = String(body?.clientDataJSON ?? '').trim()
    if (!clientDataJSON) return Respond.r400('clientDataJSON is required')
    try {
      return await Auth.passkeyRegisterFinish(token, attestationObject, clientDataJSON)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
