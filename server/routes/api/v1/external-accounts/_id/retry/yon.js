// @ts-check
import { Guards } from '../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../platform/accounts/bridge.js'
import { Respond } from '../../../../../../platform/http/respond.js'

export class Handler {
  @Guards.auth()
  static async POST(/** @type {any} */ { context, paths, body }) {
    const revision = Number(body?.revision)
    if (!Number.isSafeInteger(revision) || revision < 1) return Respond.r400('positive revision is required')
    try {
      return await ExternalAccounts.retry(bearerToken(context), String(paths?.id || ''), revision)
    } catch (error) {
      return accountFailure(error)
    }
  }
}
