import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { TemplateContent } from '../shared/mailbox/TemplateContent.js'

@RequestRoute("/templates")
@SignedIn
export class TemplatesController {
  /** @type {TemplatesHelper} */
  @Needs
  templatesHelper

  /** @param {import('../helpers/TemplatesHelper.js').TemplateEntry} entry */
  static present(entry) {
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

  /**
 * POST /templates
 * @param {object} params
 * @param {{ name: string, subject?: string, text: string, to?: string, cc?: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
    const input = TemplateContent.normalizeTemplateInput(body)
    if (input instanceof Error) return Respond.r400(input.message)

    const id = `tpl-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    const app = this
    const existing = Object.values(await app.templatesHelper.listForOwner(auth.email))
    const now = Date.now()
    const entry = {
      id,
      name: TemplateContent.resolveUniqueTemplateName(input.name, existing),
      subject: input.subject,
      text: input.text,
      to: input.to,
      cc: input.cc,
      owner: auth.email,
      createdAt: now,
      updatedAt: now,
    }
    await app.templatesHelper.put(entry)
    return TemplatesController.present(entry)
    })
  }

  /**
 * DELETE /templates
 * @param {object} params
 * @param {{ id: string }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
    const id = body?.id
    if (!id || typeof id !== 'string') return Respond.r400('id is required')

    const app = this
    const [docId, entry] = await app.templatesHelper.findById(id)
    if (!docId || !entry) return Respond.r404('Template not found')
    if (entry.owner !== auth.email) return Respond.r404('Template not found')

    await app.templatesHelper.delete(docId)
    return { deleted: true }
    })
  }

  /**
 * GET /templates
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ auth }) => {
    const app = this
    const records = await app.templatesHelper.listForOwner(auth.email)
    return Object.values(records)
      .sort((left, right) => left.name.localeCompare(right.name))
      .map(TemplatesController.present)
    })
  }
}
