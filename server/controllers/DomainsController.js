import { Guards } from '../../middleware/guards.js'
import { Actions } from '../../platform/auth/bridge.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { Security } from '../../platform/security/security.js'
import { RuleValidation } from '../../features/domains/services/rule-validation.js'
@RequestRoute("/domains")
@Public
export class DomainsController {
  /**
 * POST /domains
 * @param {object} params
 * @param {{ domain: string, routes?: object[], inboundEnabled?: boolean }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN, domain: r => r.body?.domain })
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
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
}

  /**
 * GET /domains
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { auth }) {
  const app = await getApp()
  return await app.domains.list(auth.domains)
}
}
