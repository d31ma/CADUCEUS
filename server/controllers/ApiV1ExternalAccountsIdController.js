// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/api/v1/external-accounts/{id}")
@SignedIn
export class ApiV1ExternalAccountsIdController {
  static normalizedID(value) {
    const id = typeof value === 'string' ? value.trim() : ''
    return /^ext_[A-Za-z0-9_-]{20,80}$/.test(id) ? id : ''
  }

  static normalizedRevision(value) {
    return Number.isSafeInteger(value) && value > 0 ? value : 0
  }

  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths }) => {
      const id = ApiV1ExternalAccountsIdController.normalizedID(paths?.id)
      if (!id) return Respond.r404('External account not found')
      try {
        return await ExternalAccounts.get(bearerToken(context), id)
      } catch (error) {
        return accountFailure(error)
      }
    })
  }

  @PatchRoute
  async patch(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths, body }) => {
      const id = ApiV1ExternalAccountsIdController.normalizedID(paths?.id)
      const revision = ApiV1ExternalAccountsIdController.normalizedRevision(body?.revision)
      const action = body?.action
      if (!id) return Respond.r404('External account not found')
      if (!revision || !['pause', 'resume', 'update'].includes(action)) {
        return Respond.r400('action and positive revision are required')
      }
      try {
        if (action === 'update') {
          const update = {}
          if ('label' in (body || {})) update.label = body.label
          if ('color' in (body || {})) update.color = body.color
          if ('order' in (body || {})) update.order = body.order
          return await ExternalAccounts.updatePresentation(bearerToken(context), id, revision, update)
        }
        return action === 'pause'
          ? await ExternalAccounts.pause(bearerToken(context), id, revision)
          : await ExternalAccounts.resume(bearerToken(context), id, revision)
      } catch (error) {
        return accountFailure(error)
      }
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths, body }) => {
      const id = ApiV1ExternalAccountsIdController.normalizedID(paths?.id)
      const revision = ApiV1ExternalAccountsIdController.normalizedRevision(body?.revision)
      if (!id) return Respond.r404('External account not found')
      if (!revision) return Respond.r400('positive revision is required')
      try {
        return await ExternalAccounts.remove(bearerToken(context), id, revision)
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}
