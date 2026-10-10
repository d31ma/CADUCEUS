// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/{id}/folders")
@SignedIn
export class ApiV1ExternalAccountsIdFoldersController {
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths }) => {
      try { return await ExternalAccounts.folders(ExternalAccounts.bearerToken(context), String(paths?.id || '')) }
      catch (error) { return ExternalAccounts.accountFailure(error) }
    })
  }
}
