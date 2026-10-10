// @ts-check
/**
 * Route-guard decorators for `class Handler` static methods.
 *
 * Tachyon invokes route handlers as `Handler[METHOD](request)`, where `request`
 * is `{ paths, query, body, headers, method, context }`. These TC39 method
 * decorators wrap that static method so a guard runs *before* the body: on
 * failure the decorator returns the error response and the method body never
 * executes; on success it injects the resolved principal as `request.auth` and
 * calls through.
 *
 * Every decision behind these guards is made by the Go authorization boundary.
 * A guard with no `action` proves only that the caller holds a live session; a
 * guard with one asks the boundary whether this caller may perform that action
 * on that resource, and default-deny is the boundary's answer, not this file's.
 *
 * Usage:
 *
 *   import { Guards } from './guards.js'
 *   import { Actions } from '../platform/auth/bridge.js'
 *
 *   export class Handler {
 *     // Any live session. `auth` is the resolved principal.
 *     @Guards.auth()
 *     static async GET({ paths, auth }) { ... }
 *
 *     // Authority over the caller's own mailbox.
 *     @Guards.auth({ action: Actions.MAILBOX_WRITE })
 *     static async POST({ body, auth }) { ... }
 *
 *     // Authority over a domain named by the route.
 *     @Guards.auth({ action: Actions.DOMAIN_ADMIN, domain: r => r.paths.domain })
 *     static async DELETE({ paths, auth }) { ... }
 *
 *     // Authority over a mailbox named by the request.
 *     @Guards.auth({ action: Actions.MAILBOX_READ, mailbox: r => r.query.address })
 *     static async PUT({ body, auth }) { ... }
 *   }
 *
 * `request.auth` is the {@link Principal}: the mailbox address, its domain, and
 * the identifiers of the principal, tenant, and session. Per-record checks that
 * depend on the specific document being touched stay in the method body and
 * compare against `auth.domain`.
 */

import { createHmac, timingSafeEqual } from 'node:crypto'
import { AuthGuard } from './require-auth.js'
import { Security } from '../platform/security/security.js'
import { Respond } from '../platform/http/respond.js'
import { FyloAccessContext } from '../platform/persistence/fylo-access.js'

/**
 * The shapes the authorization boundary accepts. They mirror its own rules, so
 * a value that fails here would have been refused there anyway.
 */
const DOMAIN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i
const ADDRESS = /^[a-z0-9._-]+@[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i

export class Guards {
  /**
   * Guard a static handler method behind the authorization boundary and inject
   * the resolved principal as `request.auth`.
   *
   * @param {AuthOptions} [options]
   * @returns {any}
   */
  static auth(options = {}) {
    return function (original) {
      return /** @this {any} */ async function (/** @type {HandlerRequest} */ request) {
        const result = await Guards.#resolve(request, options)
        if (!result.ok) return result.response
        // Storage ownership follows the domain the boundary authorized, not
        // the caller's own address. A member of another domain's admin group
        // acts in that domain, and the FYLO group must be that domain's.
        const domain = result.domain ?? result.principal.domain
        request.auth = {
          ...result.principal,
          email: result.principal.address,
          domain,
          domains: [domain],
        }
        return await FyloAccessContext.run(
          { email: result.principal.address, domains: [domain] },
          () => original.call(this, request),
        )
      }
    }
  }

  /**
   * @param {HandlerRequest} request
   * @param {AuthOptions} options
   * @returns {Promise<import('./require-auth.js').AuthResult>}
   */
  static async #resolve(request, options) {
    const context = request?.context ?? {}
    if (!options.action) return AuthGuard.requireAuth(context)

    // A named target comes from the request, so it is only used when it is
    // well formed. A malformed one is not an authorization question — it is a
    // bad request, and the handler says so in its own words after the guard
    // has fallen back to the caller's own least authority.
    const named = options.domain?.(request) || options.mailbox?.(request)
    if (named && options.domain && DOMAIN.test(named)) {
      return AuthGuard.requirePermission(context, options.action, { domain: named })
    }
    if (named && options.mailbox && ADDRESS.test(named)) {
      return AuthGuard.requirePermission(context, options.action, { mailbox: named })
    }

    // Nothing named, either because the route does not name a target or
    // because the request omitted it. The action then applies to whatever the
    // caller owns: its own domain for a domain action, its own mailbox
    // otherwise. That target comes from the token and is the least authority
    // available, so a missing parameter can never reach someone else's data —
    // it just leaves the handler to reject a malformed request in its own
    // words.
    const authenticated = await AuthGuard.requireAuth(context)
    if (!authenticated.ok) return authenticated
    const target = options.action.startsWith('domain:')
      ? { domain: authenticated.principal.domain }
      : { mailbox: authenticated.principal.address }
    return AuthGuard.requirePermission(context, options.action, target)
  }

  /**
   * Guard a static handler method behind an HMAC-SHA256 body signature
   * (`X-Caduceus-Signature`) computed with the named secret. Used by the
   * canonical signed inbound endpoint.
   *
   * @param {string} envName - Env var holding the signing secret.
   * @returns {any}
   */
  static webhookSignature(envName) {
    return function (original) {
      return /** @this {any} */ async function (/** @type {HandlerRequest} */ request) {
        const secret = Security.requireEnv(envName)
        const headers = request?.headers ?? {}
        const signature = headers['x-caduceus-signature'] ?? headers['X-Caduceus-Signature'] ?? ''
        const jsonBody = JSON.stringify(request?.body ?? {})
        const expected = createHmac('sha256', secret).update(jsonBody).digest('hex')
        const sigOk = signature.length === expected.length
          && timingSafeEqual(Buffer.from(signature, 'hex'), Buffer.from(expected, 'hex'))
        if (!sigOk) return Respond.r401('Invalid webhook signature')
        return await original.call(this, request)
      }
    }
  }

  /**
   * Guard a static handler method so it only runs when test routes are enabled
   * (`CADUCEUS_ENABLE_TEST_ROUTES=true` and `NODE_ENV=test`). Bare decorator.
   *
   * @param {StaticMethod} original
   * @returns {any}
   */
  static testOnly(original) {
    return /** @this {any} */ async function (/** @type {HandlerRequest} */ request) {
      if (!Security.isTestRoutesEnabled()) return Respond.r403('Only available in test mode')
      return await original.call(this, request)
    }
  }
}
