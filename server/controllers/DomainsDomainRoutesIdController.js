import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Actions } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { RuleValidation } from '../shared/domains/RuleValidation.js'

@RequestRoute("/domains/{domain}/routes/{id}")
@SignedIn
export class DomainsDomainRoutesIdController {
  /**
 * DELETE /domains/_domain/routes/_id
 * @param {object} params
 * @param {{ domain: string, id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(@FromPath domain, @FromPath id, request) {
    return RouteRequest.handle(request, { paths: { domain, id }, guard: { action: Actions.DOMAIN_ADMIN, domain: r => r.paths?.domain } }, async ({ paths, auth }) => {
    const { domain, id: ruleId } = paths ?? {}
    if (!domain || !ruleId) return Respond.r400('domain and route id required')
    if (!auth.domains.includes(domain)) return Respond.r403('Access denied')

    const app = await getApp()
    const [docId, config] = await app.domains.findEntry(domain)
    if (!docId || !config) return Respond.r404('Domain not found')

    const updated = config.routes.filter(r => r.id !== ruleId)
    await app.domains.updateRoutes(docId, updated)
    return { deleted: ruleId }
    })
  }

  /**
 * PUT /domains/_domain/routes/_id
 * @param {object} params
 * @param {{ id: string, match: string, enabled?: boolean, action?: any }} params.body
 * @param {{ domain: string, id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PutRoute
  async put(@FromPath domain, @FromPath id, request) {
    return RouteRequest.handle(request, { paths: { domain, id }, guard: { action: Actions.DOMAIN_ADMIN, domain: r => r.paths?.domain } }, async ({ paths, body, auth }) => {
    const { domain, id: ruleId } = paths ?? {}
    if (!domain || !ruleId) return Respond.r400('domain and route id required')
    if (!auth.domains.includes(domain)) return Respond.r403('Access denied')
    if (!body) return Respond.r400('Missing body')

    const actionResult = RuleValidation.validateRouteAction(body.action)
    if (!actionResult.valid) return Respond.r400(actionResult.error)

    const app = await getApp()
    const [docId, config] = await app.domains.findEntry(domain)
    if (!docId || !config) return Respond.r404('Domain not found')

    const updated = [...config.routes.filter(r => r.id !== ruleId), { ...body, id: ruleId }]
    await app.domains.updateRoutes(docId, /** @type {any} */ (updated))
    return { updated: ruleId }
    })
  }
}
