import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { Security } from '../../../platform/security/security.js'

export class Handler {
  /**
 * POST /users/aliases
 * @param {object} params
 * @param {{ alias: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  static async POST(/** @type {any} */ { body, auth }) {
  const alias = Security.normalizeEmailAddress(body?.alias)
  if (!alias) return Respond.r400('alias invalid')

  const aliasDomain = alias.split('@')[1]
  if (!Security.hasDomainClaim(auth.domains, aliasDomain)) return Respond.r403('Domain access denied')

  const app = await getApp()
  const [docId, user] = await app.users.findByEmail(auth.email)
  if (!docId || !user) return Respond.r404('User not found')
  if (!user.domains.includes(aliasDomain)) return Respond.r403('Alias domain is not assigned to this user')
  if (alias === user.email) return Respond.r400('alias must not match primary email')

  const [existingDocId] = await app.users.findByEmail(alias)
  if (existingDocId && existingDocId !== docId) return Respond.r400('alias already belongs to another user')

  const aliases = [...new Set([...(user.aliases ?? []), alias])]
  await app.users.updateAliases(docId, aliases, user.email)
  return { email: user.email, aliases }
}

  /**
 * DELETE /users/aliases
 * @param {object} params
 * @param {{ alias: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  static async DELETE(/** @type {any} */ { body, auth }) {
  const alias = Security.normalizeEmailAddress(body?.alias)
  if (!alias) return Respond.r400('alias invalid')

  const app = await getApp()
  const [docId, user] = await app.users.findByEmail(auth.email)
  if (!docId || !user) return Respond.r404('User not found')

  const aliases = (user.aliases ?? []).filter(item => item !== alias)
  if (aliases.length === (user.aliases ?? []).length) return Respond.r404('Alias not found')

  await app.users.updateAliases(docId, aliases, user.email)
  return { email: user.email, aliases }
}

  /**
 * GET /users/aliases
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth()
  static async GET(/** @type {any} */ { auth }) {
  const app = await getApp()
  const [, user] = await app.users.findByEmail(auth.email)
  if (!user) return Respond.r404('User not found')
  return { email: user.email, aliases: user.aliases ?? [] }
}
}
