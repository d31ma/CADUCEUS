// @ts-check
import { RouteRequest } from '../middleware/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'
import { Respond } from '../platform/http/Respond.js'

@RequestRoute("/api/v1/external-accounts/{id}/retry")
@SignedIn
export class ApiV1ExternalAccountsIdRetryController {
  @PostRoute
  async post(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths, body }) => {
      const revision = Number(body?.revision)
      if (!Number.isSafeInteger(revision) || revision < 1) return Respond.r400('positive revision is required')
      try {
        return await ExternalAccounts.retry(bearerToken(context), String(paths?.id || ''), revision)
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
