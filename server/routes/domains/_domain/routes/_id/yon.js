import { Guards } from '../../../../../middleware/guards.js'
import { Actions } from '../../../../../platform/auth/bridge.js'
import { Respond } from '../../../../../platform/http/respond.js'
import { getApp } from '../../../../../app.js'
import { RuleValidation } from '../../../../../features/domains/services/rule-validation.js'

export class Handler {
  /**
 * DELETE /domains/_domain/routes/_id
 * @param {object} params
 * @param {{ domain: string, id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN, domain: r => r.paths?.domain })
  static async DELETE(/** @type {any} */ { paths, auth }) {
  const { domain, id: ruleId } = paths ?? {}
  if (!domain || !ruleId) return Respond.r400('domain and route id required')
  if (!auth.domains.includes(domain)) return Respond.r403('Access denied')

  const app = await getApp()
  const [docId, config] = await app.domains.findEntry(domain)
  if (!docId || !config) return Respond.r404('Domain not found')

  const updated = config.routes.filter(r => r.id !== ruleId)
  await app.domains.updateRoutes(docId, updated)
  return { deleted: ruleId }
}

  /**
 * PUT /domains/_domain/routes/_id
 * @param {object} params
 * @param {{ id: string, match: string, enabled?: boolean, action?: any }} params.body
 * @param {{ domain: string, id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.DOMAIN_ADMIN, domain: r => r.paths?.domain })
  static async PUT(/** @type {any} */ { paths, body, auth }) {
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
}
}
