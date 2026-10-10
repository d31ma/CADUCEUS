// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/{id}/reconnect")
@SignedIn
export class ApiV1ExternalAccountsIdReconnectController {
  @PostRoute
  async post(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths, body }) => {
      const id = typeof paths?.id === 'string' && /^ext_[A-Za-z0-9_-]{20,80}$/.test(paths.id) ? paths.id : ''
      const revision = Number.isSafeInteger(body?.revision) && body.revision > 0 ? body.revision : 0
      const credentials = body?.credentials
      if (!id) return Respond.r404('External account not found')
      if (!revision || !credentials || typeof credentials !== 'object' || Array.isArray(credentials)) {
        return Respond.r400('credentials and positive revision are required')
      }
      try {
        return await ExternalAccounts.reconnect(ExternalAccounts.bearerToken(context), id, revision, credentials)
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }
}
