import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { Security } from '../shared/security/Security.js'

@RequestRoute("/users")
@SignedIn
export class UsersController {
  /**
 * POST /users
 * @param {object} params
 * @param {{ email: string, domains: string[], role: string, aliases?: string[] }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.DOMAIN_ADMIN, domain: r => r.body?.domains?.[0] } }, async ({ body, auth }) => {
    const user = body
    if (!user?.email) return Respond.r400('email required')
    if (!user.domains?.length) return Respond.r400('domains required')
    if (!user.role) return Respond.r400('role required')
    if (user.role !== 'admin' && user.role !== 'viewer') return Respond.r400('role invalid')

    const email = Security.normalizeEmailAddress(user.email)
    if (!email) return Respond.r400('email invalid')

    const aliases = (user.aliases ?? []).map(alias => Security.normalizeEmailAddress(alias))
    if (aliases.some(alias => !alias)) return Respond.r400('aliases invalid')

    const domains = user.domains.map(domain => Security.normalizeDomain(domain))
    if (domains.some(domain => !domain)) return Respond.r400('domains invalid')

    const normalizedAliases = aliases.filter(/** @returns {a is string} */ a => Boolean(a))
    if (normalizedAliases.includes(email)) return Respond.r400('aliases must not include primary email')
    if (!normalizedAliases.every(alias => domains.includes(alias.split('@')[1]))) {
      return Respond.r400('aliases must belong to user domains')
    }
    if (!domains.every(domain => Security.hasDomainClaim(auth.domains, /** @type {string} */ (domain)))) {
      return Respond.r403('Domain access denied')
    }

    const app = await getApp()
    for (const address of [email, ...normalizedAliases]) {
      const [existingDocId, existingUser] = await app.users.findByEmail(address)
      if (existingDocId && existingUser?.email !== email) {
        return Respond.r400(`address already belongs to another user: ${address}`)
      }
    }

    await app.users.put(/** @type {any} */ ({
      email,
      aliases: normalizedAliases,
      domains,
      role: user.role,
    }))
    return { email }
    })
  }

  /**
 * GET /users
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.DOMAIN_ADMIN } }, async ({ auth }) => {
    const app = await getApp()
    return await app.users.list()
    })
  }
}
