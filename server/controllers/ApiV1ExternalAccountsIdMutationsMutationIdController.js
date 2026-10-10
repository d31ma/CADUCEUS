// @ts-check
import { RouteRequest } from '../middleware/RouteRequest.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'
import { Respond } from '../platform/http/Respond.js'

@RequestRoute("/api/v1/external-accounts/{id}/mutations/{mutation_id}")
@SignedIn
export class ApiV1ExternalAccountsIdMutationsMutationIdController {
  @GetRoute
  async get(@FromPath id, @FromPath mutation_id, request) {
    return RouteRequest.handle(request, { paths: { id, mutation_id }, guard: {} }, async ({ context, paths }) => {
      const id = String(paths?.id || '')
      const mutationId = String(paths?.mutation_id || '')
      if (!id || !mutationId) return Respond.r404('Provider mutation not found')
      try {
        return await ExternalAccounts.mutation(bearerToken(context), id, mutationId)
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
