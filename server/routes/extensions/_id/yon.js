import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { getApp } from '../../../app.js'
import { presentExtension } from '../../../features/extensions/services/extensions.js'

export class Handler {
  @Guards.auth({ action: Actions.MAILBOX_READ })
  static async GET(/** @type {any} */ { auth, paths }) {
    const record = await ownedRecord(auth.email, paths?.id)
    if (!record) return Respond.r404('Extension not found')
    return presentExtension(record, { runtime: true })
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async PUT(/** @type {any} */ { auth, paths, body }) {
    if (!body || typeof body !== 'object' || Array.isArray(body) || Object.keys(body).length !== 1 || typeof body.enabled !== 'boolean') return Respond.r400('enabled must be the only field')
    const app = await getApp()
    const record = await ownedRecord(auth.email, paths?.id)
    if (!record) return Respond.r404('Extension not found')
    const now = new Date().toISOString()
    if (body.enabled && record.kind === 'theme') {
      for (const other of await app.extensions.listForOwner(auth.email)) {
        if (other.kind === 'theme' && other.enabled && other.docId !== record.docId) await app.extensions.update(other.docId, { enabled: false, updatedAt: now })
      }
    }
    await app.extensions.update(record.docId, { enabled: body.enabled, updatedAt: now })
    return presentExtension({ ...record, enabled: body.enabled, updatedAt: now })
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { auth, paths }) {
    const app = await getApp()
    const record = await ownedRecord(auth.email, paths?.id)
    if (!record) return Respond.r404('Extension not found')
    await app.extensions.delete(record.docId)
    return { deleted: true }
  }
}

async function ownedRecord(owner, id) {
  const extensionId = typeof id === 'string' ? id : ''
  if (!extensionId) return null
  const app = await getApp()
  return await app.extensions.findForOwner(owner, extensionId)
}
