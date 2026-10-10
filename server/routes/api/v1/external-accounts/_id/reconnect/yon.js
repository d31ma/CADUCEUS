// @ts-check
import { Guards } from '../../../../../../middleware/guards.js'
import { Respond } from '../../../../../../platform/http/respond.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async POST(/** @type {any} */ { context, paths, body }) {
    const id = typeof paths?.id === 'string' && /^ext_[A-Za-z0-9_-]{20,80}$/.test(paths.id) ? paths.id : ''
    const revision = Number.isSafeInteger(body?.revision) && body.revision > 0 ? body.revision : 0
    const credentials = body?.credentials
    if (!id) return Respond.r404('External account not found')
    if (!revision || !credentials || typeof credentials !== 'object' || Array.isArray(credentials)) {
      return Respond.r400('credentials and positive revision are required')
    }
    try {
      return await ExternalAccounts.reconnect(bearerToken(context), id, revision, credentials)
    } catch (error) {
      return accountFailure(error)
    }
  }
}
