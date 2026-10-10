import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { ownedSenderAddresses } from '../../../features/users/services/sender-addresses.js'
import { normalizeSignatureInput, resolveUniqueSignatureName } from '../../../features/mailbox/services/signature-content.js'

/** @param {import('../../../features/mailbox/repositories/signatures.js').SignatureEntry} entry */
function present(entry) {
  return {
    id: entry.id,
    domain: entry.domain,
    name: entry.name,
    text: entry.text,
    defaultFor: entry.defaultFor ?? [],
    createdAt: new Date(entry.createdAt).toISOString(),
    updatedAt: new Date(entry.updatedAt || entry.createdAt).toISOString(),
  }
}

export class Handler {
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async PUT(/** @type {any} */ { paths, body, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Signature not found')
    const app = await getApp()
    const [docId, existing] = await app.signatures.findById(id)
    if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Signature not found')
    const addresses = await ownedSenderAddresses(app.users, auth.email)
    const input = normalizeSignatureInput(body, { partial: true, allowedAddresses: addresses })
    if (input instanceof Error) return Respond.r400(input.message)
    if (input.name) {
      input.name = resolveUniqueSignatureName(input.name, Object.values(await app.signatures.listForOwner(auth.email)), id)
    }
    const patch = { ...input, updatedAt: Date.now() }
    if (input.defaultFor) await app.signatures.releaseDefaults(auth.email, input.defaultFor, id)
    await app.signatures.update(docId, patch)
    return present({ ...existing, ...patch })
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { paths, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Signature not found')
    const app = await getApp()
    const [docId, existing] = await app.signatures.findById(id)
    if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Signature not found')
    await app.signatures.delete(docId)
    return { deleted: id }
  }
}
