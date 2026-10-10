import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Security } from '../shared/security/Security.js'

@RequestRoute("/users/aliases")
@SignedIn
export class UsersAliasesController {
  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /**
 * POST /users/aliases
 * @param {object} params
 * @param {{ alias: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
    const alias = Security.normalizeEmailAddress(body?.alias)
    if (!alias) return Respond.r400('alias invalid')

    const aliasDomain = alias.split('@')[1]
    if (!Security.hasDomainClaim(auth.domains, aliasDomain)) return Respond.r403('Domain access denied')

    const app = this
    const [docId, user] = await app.usersHelper.findByEmail(auth.email)
    if (!docId || !user) return Respond.r404('User not found')
    if (!user.domains.includes(aliasDomain)) return Respond.r403('Alias domain is not assigned to this user')
    if (alias === user.email) return Respond.r400('alias must not match primary email')

    const [existingDocId] = await app.usersHelper.findByEmail(alias)
    if (existingDocId && existingDocId !== docId) return Respond.r400('alias already belongs to another user')

    const aliases = [...new Set([...(user.aliases ?? []), alias])]
    await app.usersHelper.updateAliases(docId, aliases, user.email)
    return { email: user.email, aliases }
    })
  }

  /**
 * DELETE /users/aliases
 * @param {object} params
 * @param {{ alias: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
    const alias = Security.normalizeEmailAddress(body?.alias)
    if (!alias) return Respond.r400('alias invalid')

    const app = this
    const [docId, user] = await app.usersHelper.findByEmail(auth.email)
    if (!docId || !user) return Respond.r404('User not found')

    const aliases = (user.aliases ?? []).filter(item => item !== alias)
    if (aliases.length === (user.aliases ?? []).length) return Respond.r404('Alias not found')

    await app.usersHelper.updateAliases(docId, aliases, user.email)
    return { email: user.email, aliases }
    })
  }

  /**
 * GET /users/aliases
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = this
    const [, user] = await app.usersHelper.findByEmail(auth.email)
    if (!user) return Respond.r404('User not found')
    return { email: user.email, aliases: user.aliases ?? [] }
    })
  }
}
