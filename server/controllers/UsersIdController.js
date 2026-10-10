import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/users/{id}")
@SignedIn
export class UsersIdController {
  /** @type {UsersHelper} */
  @Needs
  usersHelper

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

    const app = this
    const [docId] = await app.usersHelper.findByEmail(email)
    if (!docId) return Respond.r404('User not found')

    await app.usersHelper.delete(docId)
    return { deleted: email }
    })
  }
}
