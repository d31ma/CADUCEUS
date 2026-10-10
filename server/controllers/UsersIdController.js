import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/users/{id}")
@SignedIn
export class UsersIdController {
  /**
 * DELETE /users/_id
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.DOMAIN_ADMIN, domain: r => r.body?.domains?.[0] } }, async ({ paths, auth }) => {
    const email = paths?.id
    if (!email) return Respond.r400('email required')

    const app = await getApp()
    const [docId] = await app.users.findByEmail(email)
    if (!docId) return Respond.r404('User not found')

    await app.users.delete(docId)
    return { deleted: email }
    })
  }
}
