// @ts-check
import { Auth, AuthBridgeError, Actions } from '../auth/Auth.js'
import { Respond } from './Respond.js'
/** @import { Principal } from '../../models/Principal.js' */
/** @import { AuthSuccess } from '../../models/AuthSuccess.js' */
/** @import { AuthFailure } from '../../models/AuthFailure.js' */
/** @import { AuthResult } from '../../models/AuthResult.js' */

/**
 * Bearer-token checks against the Go authorization boundary.
 *
 * Nothing here decides anything. It carries the caller's token to the boundary
 * and turns the boundary's answer into an HTTP response. A route cannot reach
 * a mailbox without an answer, which is what makes the boundary unbypassable
 * rather than merely conventional.
 */
export class AuthGuard {
  /**
   * Verify the bearer token and resolve the caller.
   *
   * @param {{ bearer?: { token: string } }} context
   * @returns {Promise<AuthResult>}
   */
  static async requireAuth(context) {
    const token = context?.bearer?.token
    if (!token) return { ok: false, response: Respond.r401('Authentication required') }
    try {
      return { ok: true, principal: await Auth.authenticate(token) }
    } catch (error) {
      return { ok: false, response: AuthGuard.failure(error) }
    }
  }

  /**
   * Verify the caller and ask the boundary whether the request is permitted.
   *
   * @param {{ bearer?: { token: string } }} context
   * @param {string} action
   * @param {{ mailbox?: string, domain?: string, resource?: string }} target
   * @returns {Promise<AuthResult>}
   */
  static async requirePermission(context, action, target) {
    const token = context?.bearer?.token
    if (!token) return { ok: false, response: Respond.r401('Authentication required') }
    try {
      const decision = await Auth.authorize(token, action, target)
      if (!decision.allowed) return { ok: false, response: Respond.r403('Access denied') }
      return { ok: true, principal: decision.principal, domain: decision.domain }
    } catch (error) {
      return { ok: false, response: AuthGuard.failure(error) }
    }
  }

  /**
   * Require read access to one mail domain's configuration.
   *
   * @param {{ bearer?: { token: string } }} context
   * @param {string} domain
   * @returns {Promise<AuthResult>}
   */
  static requireDomainAccess(context, domain) {
    return AuthGuard.requirePermission(context, Actions.DOMAIN_READ, { domain })
  }

  /**
   * Require administrative authority over one mail domain.
   *
   * @param {{ bearer?: { token: string } }} context
   * @param {string} domain
   * @returns {Promise<AuthResult>}
   */
  static requireDomainAdmin(context, domain) {
    return AuthGuard.requirePermission(context, Actions.DOMAIN_ADMIN, { domain })
  }

  /**
   * Require authority over one mailbox.
   *
   * @param {{ bearer?: { token: string } }} context
   * @param {string} action
   * @param {string} mailbox
   * @returns {Promise<AuthResult>}
   */
  static requireMailboxAccess(context, action, mailbox) {
    return AuthGuard.requirePermission(context, action, { mailbox })
  }

  /**
   * Map a boundary failure onto a response. An unrecognized failure is never
   * treated as permission to continue.
   *
   * @param {unknown} error
   * @returns {Record<string, unknown>}
   */
  static failure(error) {
    if (error instanceof AuthBridgeError) {
      switch (error.status) {
        case 400: return Respond.r400(error.message)
        case 401: return Respond.r401('Authentication required')
        case 403: return Respond.r403(error.code === 'EPOLICY' ? error.message : 'Access denied')
        case 409: return Respond.r409(error.message)
        default:  return Respond.r503('Authentication is unavailable')
      }
    }
    return Respond.r503('Authentication is unavailable')
  }
}

export { Actions }
