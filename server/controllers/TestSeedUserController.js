// @ts-check
import { randomBytes } from 'node:crypto'
import { Respond } from '../platform/http/Respond.js'
import { RouteRequest } from '../middleware/RouteRequest.js'
import { AuthGuard } from '../middleware/AuthGuard.js'
import { authCall, Auth } from '../platform/auth/Auth.js'
import { getApp } from '../app.js'

@RequestRoute("/test/seed/user")
@Public
export class TestSeedUserController {
  /**
   * POST /test/seed/user
   *
   * Provision one mailbox and return a real token for it.
   *
   * The token comes from the engine, through the same login the product uses:
   * provision the mailbox, take its bootstrap credentials, and complete an
   * authentication with them. A harness that could mint its own token would
   * prove nothing about the boundary that verifies them, so it does not.
   *
   * Only enabled in test mode — production configuration rejects both this
   * route and the bootstrap operation behind it.
   *
   * @param {any} request
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { testOnly: true }, async ({ body }) => {
      const user = body
      if (!user?.email || !user.domains?.length || !user.role) {
        return Respond.r400('email, domains, and role required')
      }
      const address = String(user.email).toLowerCase()
      const role = user.role === 'admin' ? 'domain-admin' : 'domain-viewer'

      // Each seeded mailbox gets its own password; the response hands it back.
      const password = randomBytes(24).toString('base64url')
      try {
        const seeded = await authCall('auth.test.bootstrap', {
          address, password, role,
        })
        const tokens = await TestSeedUserController._login(address, password, seeded.recoveryCodes[0])

        // The mail-side profile is CADUCEUS's own record of the mailbox.
        // Identity lives in the engine; this is the address-book entry beside it.
        const app = await getApp()
        await app.users.upsertByEmail(/** @type {any} */ ({
          ...user,
          email: address,
          principalId: seeded.principalId,
          tenantId: seeded.tenantId,
        }))

        return {
          email: address,
          password,
          name: user.name || null,
          role: user.role,
          domains: [seeded.domain],
          token: tokens.accessToken,
          refreshToken: tokens.refreshToken,
          idToken: tokens.idToken,
          recoveryCodes: seeded.recoveryCodes.slice(1),
        }
      } catch (error) {
        return AuthGuard.failure(error)
      }
    })
  }

  /**
   * Drive one complete authentication: password, then a recovery code as the
   * second factor, then completion.
   *
   * @param {string} address
   * @param {string} password
   * @param {string} recoveryCode
   */
  static async _login(address, password, recoveryCode) {
    const begun = await Auth.loginBegin(address)
    await Auth.verifyPassword(begun.transactionId, password)
    await Auth.verifyRecoveryCode(begun.transactionId, recoveryCode)
    return await Auth.completeLogin(begun.transactionId)
  }
}
