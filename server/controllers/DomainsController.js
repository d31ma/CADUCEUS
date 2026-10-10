import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { Security } from '../shared/security/Security.js'
import { RuleValidation } from '../shared/domains/RuleValidation.js'

@RequestRoute("/domains")
@SignedIn
export class DomainsController {
  /**
 * POST /domains
 * @param {object} params
 * @param {{ domain: string, routes?: object[], inboundEnabled?: boolean }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.DOMAIN_ADMIN, domain: r => r.body?.domain } }, async ({ body, auth }) => {
    const config = body
    if (!config?.domain) return Respond.r400('domain required')

    const domain = Security.normalizeDomain(config.domain)
    if (!domain) return Respond.r400('domain invalid')
    if (!Security.hasDomainClaim(auth.domains, domain)) return Respond.r403('Domain access denied')

    const routes = /** @type {any} */ (config.routes ?? [])
    const routeResult = RuleValidation.validateRouteRules(routes)
    if (!routeResult.valid) return Respond.r400(routeResult.error)

    const app = await getApp()
    await app.domains.put({
      domain,
      routes,
      inboundEnabled: config.inboundEnabled ?? false,
    })
    return { domain }
    })
  }

  /**
 * GET /domains
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = await getApp()
    return await app.domains.list(auth.domains)
    })
  }
}
