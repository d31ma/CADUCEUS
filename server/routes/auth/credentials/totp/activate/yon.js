// @ts-check
import { Respond } from '../../../../../platform/http/respond.js'
import { Auth } from '../../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../../middleware/require-auth.js'

export class Handler {
  /**
   * POST /auth/credentials/totp/activate — prove a code and enable TOTP.
   *
   * @param {any} request
   */
  static async POST(/** @type {any} */ { body, context }) {
    const token = context?.bearer?.token
    if (!token) return Respond.r401('Authentication required')
    const code = String(body?.code ?? '').trim()
    if (!code) return Respond.r400('code is required')
    try {
      return await Auth.totpActivate(token, code)
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
