import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { Extensions } from '../shared/extensions/Extensions.js'

@RequestRoute("/extensions/{id}")
@SignedIn
export class ExtensionsIdController {
  /** @type {ExtensionsHelper} */
  @Needs
  extensionsHelper

  async ownedRecord(owner, id) {
    const extensionId = typeof id === 'string' ? id : ''
    if (!extensionId) return null
    const app = this
    return await app.extensionsHelper.findForOwner(owner, extensionId)
  }

  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ auth, paths }) => {
      const record = await this.ownedRecord(auth.email, paths?.id)
      if (!record) return Respond.r404('Extension not found')
      return Extensions.presentExtension(record, { runtime: true })
    })
  }

  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ auth, paths, body }) => {
      if (!body || typeof body !== 'object' || Array.isArray(body) || Object.keys(body).length !== 1 || typeof body.enabled !== 'boolean') return Respond.r400('enabled must be the only field')
      const app = this
      const record = await this.ownedRecord(auth.email, paths?.id)
      if (!record) return Respond.r404('Extension not found')
      const now = new Date().toISOString()
      if (body.enabled && record.kind === 'theme') {
        for (const other of await app.extensionsHelper.listForOwner(auth.email)) {
          if (other.kind === 'theme' && other.enabled && other.docId !== record.docId) await app.extensionsHelper.update(other.docId, { enabled: false, updatedAt: now })
        }
      }
      await app.extensionsHelper.update(record.docId, { enabled: body.enabled, updatedAt: now })
      return Extensions.presentExtension({ ...record, enabled: body.enabled, updatedAt: now })
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ auth, paths }) => {
      const app = this
      const record = await this.ownedRecord(auth.email, paths?.id)
      if (!record) return Respond.r404('Extension not found')
      await app.extensionsHelper.delete(record.docId)
      return { deleted: true }
    })
  }
}
