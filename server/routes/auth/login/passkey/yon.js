// @ts-check
import { Respond } from '../../../../platform/http/respond.js'
import { Auth } from '../../../../platform/auth/bridge.js'
import { AuthGuard } from '../../../../middleware/require-auth.js'
import { getApp } from '../../../../app.js'

export class Handler {
  /**
   * POST /auth/login/passkey — supply a WebAuthn assertion.
   *
   * This route holds no authentication logic. It validates the shape of the
   * request, carries it to the Go boundary, and reports the answer.
   *
   * @param {any} request
   */
  static async POST(/** @type {any} */ { body, context }) {
    const transactionId = String(body?.transactionId ?? '').trim()
    if (!transactionId) return Respond.r400('transactionId is required')
    const credentialId = String(body?.credentialId ?? '').trim()
    if (!credentialId) return Respond.r400('credentialId is required')
    const authenticatorData = String(body?.authenticatorData ?? '').trim()
    if (!authenticatorData) return Respond.r400('authenticatorData is required')
    const clientDataJSON = String(body?.clientDataJSON ?? '').trim()
    if (!clientDataJSON) return Respond.r400('clientDataJSON is required')
    const signature = String(body?.signature ?? '').trim()
    if (!signature) return Respond.r400('signature is required')

    const app = await getApp()
    const limit = await app.rateLimit.check(
      ['login-passkey', transactionId, context?.ipAddress ?? 'unknown'], 12, 15 * 60 * 1000,
    )
    if (!limit.allowed) return Respond.r429('Too many attempts', limit.retryAfterSeconds)

    try {
      return await Auth.verifyPasskey({ transactionId, credentialId, authenticatorData, clientDataJSON, signature })
    } catch (error) {
      return AuthGuard.failure(error)
    }
  }
}
