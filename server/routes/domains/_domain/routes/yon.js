import { Guards } from '../../../../middleware/guards.js'
import { Respond } from '../../../../platform/http/respond.js'
import { getApp } from '../../../../app.js'

export class Handler {
  /**
 * GET /domains/_domain/routes
 * @param {object} params
 * @param {{ domain: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @Guards.auth()
  static async GET(/** @type {any} */ { paths, auth }) {
  const domain = paths?.domain
  if (!domain) return Respond.r404('Domain not found')
  if (!auth.domains.includes(domain)) return Respond.r403('Access denied')

  const app = await getApp()
  const [, config] = await app.domains.findEntry(domain)
  if (!config) return Respond.r404('Domain not found')
  return config.routes
}
}
