import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/domains/{domain}/routes")
@SignedIn
export class DomainsDomainRoutesController {
  /** @type {DomainsHelper} */
  @Needs
  domainsHelper

  /**
 * GET /domains/_domain/routes
 * @param {object} params
 * @param {{ domain: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(@FromPath domain, request) {
    return RouteRequest.handle(request, { paths: { domain }, guard: {} }, async ({ paths, auth }) => {
    const domain = paths?.domain
    if (!domain) return Respond.r404('Domain not found')
    if (!auth.domains.includes(domain)) return Respond.r403('Access denied')

    const app = this
    const [, config] = await app.domainsHelper.findEntry(domain)
    if (!config) return Respond.r404('Domain not found')
    return config.routes
    })
  }
}
