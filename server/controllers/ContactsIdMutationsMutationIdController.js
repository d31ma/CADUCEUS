import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { AuthBridgeError } from '../platform/auth/Auth.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/contacts/{id}/mutations/{mutation_id}")
@SignedIn
export class ContactsIdMutationsMutationIdController {
  @GetRoute
  async get(@FromPath id, @FromPath mutation_id, request) {
    return RouteRequest.handle(request, { paths: { id, mutation_id }, guard: {} }, async ({ paths, query, context }) => {
      const accountId = String(query?.accountId || '')
  	const contactId = String(paths?.id || '')
      const mutationId = String(paths?.mutation_id || '')
  	if (!accountId || !contactId || !mutationId) return Respond.r400('accountId, contact id, and mutation id are required')
      try {
  	  const mutation = await ExternalAccounts.contactMutation(bearerToken(context), accountId, mutationId)
  	  if (mutation?.contactId !== contactId) return Respond.r404('Contact mutation not found')
  	  return { ...mutation, lastCode: mutation?.lastCode || '' }
  	} catch (error) {
  	  if (error instanceof AuthBridgeError) return accountFailure(error)
        return Respond.r404('Contact mutation not found')
      }
    })
  }
}
