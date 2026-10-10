import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { normalizeTemplateInput, resolveUniqueTemplateName } from '../../../features/mailbox/services/template-content.js'

/** @param {import('../../../features/mailbox/repositories/templates.js').TemplateEntry} entry */
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
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async PUT(/** @type {any} */ { paths, body, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Template not found')
    const input = normalizeTemplateInput(body, { partial: true })
    if (input instanceof Error) return Respond.r400(input.message)

    const app = await getApp()
    const [docId, existing] = await app.templates.findById(id)
    if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Template not found')
    if (input.name) {
      const records = Object.values(await app.templates.listForOwner(auth.email))
      input.name = resolveUniqueTemplateName(input.name, records, id)
    }
    const patch = { ...input, updatedAt: Date.now() }
    await app.templates.update(docId, patch)
    return present({ ...existing, ...patch })
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { paths, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Template not found')
    const app = await getApp()
    const [docId, existing] = await app.templates.findById(id)
    if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Template not found')
    await app.templates.delete(docId)
    return { deleted: id }
  }
}
