import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { parseDraftInput } from '../validation.js'
import { ownedSenderAddresses, resolveOwnedSender } from '../../../features/users/services/sender-addresses.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { paths, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Draft not found')
    const app = await getApp()
    return await app.drafts.get(id, auth.email, auth.domains) ?? Respond.r404('Draft not found')
  }

  @Guards.auth()
  static async PUT(/** @type {any} */ { paths, body, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Draft not found')
    if (!Number.isInteger(body?.version) || body.version < 1) return Respond.r400('version is required')
    const input = parseDraftInput(body)
    if (input instanceof Error) return Respond.r400(input.message)
    const app = await getApp()
    const addresses = await ownedSenderAddresses(app.users, auth.email)
    const sender = resolveOwnedSender(input.from, addresses, auth.email)
    if (!sender) return Respond.r403('From address is not owned by this mailbox')
    input.from = sender
    const result = await app.drafts.update(id, auth.email, auth.domains, body.version, input)
    if (result.kind === 'not-found') return Respond.r404('Draft not found')
    if (result.kind === 'conflict') {
      return {
        error: 'Draft changed in another session',
        conflict: true,
        currentVersion: result.currentVersion,
        updatedAt: result.updatedAt,
      }
    }
    return result.draft
  }

  @Guards.auth()
  static async DELETE(/** @type {any} */ { paths, auth }) {
    const id = String(paths?.id || '')
    if (!id) return Respond.r404('Draft not found')
    const app = await getApp()
    if (!await app.drafts.discard(id, auth.email, auth.domains)) return Respond.r404('Draft not found')
    return { discarded: id }
  }
}
