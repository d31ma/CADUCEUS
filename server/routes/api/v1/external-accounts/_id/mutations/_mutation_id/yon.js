// @ts-check
import { Guards } from '../../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../../platform/accounts/bridge.js'
import { Respond } from '../../../../../../../platform/http/respond.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { context, paths }) {
    const id = String(paths?.id || '')
    const mutationId = String(paths?.mutation_id || '')
    if (!id || !mutationId) return Respond.r404('Provider mutation not found')
    try {
      return await ExternalAccounts.mutation(bearerToken(context), id, mutationId)
    } catch (error) {
      return accountFailure(error)
    }
  }
}
