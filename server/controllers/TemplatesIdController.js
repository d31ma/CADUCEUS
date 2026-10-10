import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { Auth } from '../shared/auth/Auth.js'
import { TemplateContent } from '../shared/mailbox/TemplateContent.js'

@RequestRoute("/templates/{id}")
@SignedIn
export class TemplatesIdController {
  /** @param {import('../features/mailbox/repositories/TemplatesRepository.js').TemplateEntry} entry */
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

  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, body, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Template not found')
      const input = TemplateContent.normalizeTemplateInput(body, { partial: true })
      if (input instanceof Error) return Respond.r400(input.message)

      const app = await getApp()
      const [docId, existing] = await app.templates.findById(id)
      if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Template not found')
      if (input.name) {
        const records = Object.values(await app.templates.listForOwner(auth.email))
        input.name = TemplateContent.resolveUniqueTemplateName(input.name, records, id)
      }
      const patch = { ...input, updatedAt: Date.now() }
      await app.templates.update(docId, patch)
      return TemplatesIdController.present({ ...existing, ...patch })
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Template not found')
      const app = await getApp()
      const [docId, existing] = await app.templates.findById(id)
      if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Template not found')
      await app.templates.delete(docId)
      return { deleted: id }
    })
  }
}
