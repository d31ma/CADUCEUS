// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/api/v1/external-accounts/{id}/mutations")
@SignedIn
export class ApiV1ExternalAccountsIdMutationsController {
  @PostRoute
  async post(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ context, paths, body }) => {
      const revision = Number(body?.revision)
      const kind = String(body?.kind || '')
      const idempotencyKey = String(body?.idempotencyKey || '').trim()
      const messageStableIds = Array.isArray(body?.messageStableIds) ? body.messageStableIds : []
      const messageKinds = ['set_read', 'set_starred', 'move', 'delete']
      const folderKinds = ['folder_create', 'folder_rename', 'folder_delete']
      if (!Number.isSafeInteger(revision) || revision < 1 || !idempotencyKey || ![...messageKinds, ...folderKinds].includes(kind) || messageKinds.includes(kind) && !messageStableIds.length) {
        return Respond.r400('revision, idempotencyKey, kind, and the required mutation target are required')
      }
      try {
        return await ExternalAccounts.mutate(ExternalAccounts.bearerToken(context), String(paths?.id || ''), {
          revision, idempotencyKey, kind, messageStableIds,
          targetFolderId: String(body?.targetFolderId || ''), value: body?.value === true,
          folderStableId: String(body?.folderStableId || ''), parentStableId: String(body?.parentStableId || ''),
          folderName: String(body?.folderName || ''),
        })
      } catch (error) { return ExternalAccounts.accountFailure(error) }
    })
  }
}
