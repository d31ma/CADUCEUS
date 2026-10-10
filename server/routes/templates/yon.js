import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { Actions } from '../../platform/auth/bridge.js'
import { normalizeTemplateInput, resolveUniqueTemplateName } from '../../features/mailbox/services/template-content.js'

/** @param {import('../../features/mailbox/repositories/templates.js').TemplateEntry} entry */
function present(entry) {
  return {
    id: entry.id,
    name: entry.name,
    subject: entry.subject,
    text: entry.text,
    to: entry.to,
    cc: entry.cc,
    createdAt: new Date(entry.createdAt).toISOString(),
    updatedAt: new Date(entry.updatedAt || entry.createdAt).toISOString(),
  }
}

export class Handler {
  /**
 * POST /templates
 * @param {object} params
 * @param {{ name: string, subject?: string, text: string, to?: string, cc?: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async POST(/** @type {any} */ { body, auth }) {
  const input = normalizeTemplateInput(body)
  if (input instanceof Error) return Respond.r400(input.message)

  const id = `tpl-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
  const app = await getApp()
  const existing = Object.values(await app.templates.listForOwner(auth.email))
  const now = Date.now()
  const entry = {
    id,
    name: resolveUniqueTemplateName(input.name, existing),
    subject: input.subject,
    text: input.text,
    to: input.to,
    cc: input.cc,
    owner: auth.email,
    createdAt: now,
    updatedAt: now,
  }
  await app.templates.put(entry)
  return present(entry)
}

  /**
 * DELETE /templates
 * @param {object} params
 * @param {{ id: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { body, auth }) {
  const id = body?.id
  if (!id || typeof id !== 'string') return Respond.r400('id is required')

  const app = await getApp()
  const [docId, entry] = await app.templates.findById(id)
  if (!docId || !entry) return Respond.r404('Template not found')
  if (entry.owner !== auth.email) return Respond.r404('Template not found')

  await app.templates.delete(docId)
  return { deleted: true }
}

  /**
 * GET /templates
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @Guards.auth({ action: Actions.MAILBOX_READ })
  static async GET(/** @type {any} */ { auth }) {
  const app = await getApp()
  const records = await app.templates.listForOwner(auth.email)
  return Object.values(records)
    .sort((left, right) => left.name.localeCompare(right.name))
    .map(present)
}
}
