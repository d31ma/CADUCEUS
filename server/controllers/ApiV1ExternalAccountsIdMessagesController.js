// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/{id}/messages")
@SignedIn
export class ApiV1ExternalAccountsIdMessagesController {
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths, query }) => {
      const limit = Math.max(1, Math.min(500, Number(query?.limit) || 50))
      try { return await ExternalAccounts.messages(bearerToken(context), String(paths?.id || ''), String(query?.folder || ''), limit) }
      catch (error) { return accountFailure(error) }
    })
  }
}
