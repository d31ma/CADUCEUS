// @ts-check
import { RouteRequest } from '../middleware/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/{id}/folders")
@SignedIn
export class ApiV1ExternalAccountsIdFoldersController {
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths }) => {
      try { return await ExternalAccounts.folders(bearerToken(context), String(paths?.id || '')) }
      catch (error) { return accountFailure(error) }
    })
  }
}
