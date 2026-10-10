// @ts-check
import { Guards } from '../../../../../../middleware/guards.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../../platform/accounts/bridge.js'
import { Respond } from '../../../../../../platform/http/respond.js'

export class Handler {
  @Guards.auth()
  static async POST(/** @type {any} */ { context, paths, body }) {
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
      return await ExternalAccounts.mutate(bearerToken(context), String(paths?.id || ''), {
        revision, idempotencyKey, kind, messageStableIds,
        targetFolderId: String(body?.targetFolderId || ''), value: body?.value === true,
        folderStableId: String(body?.folderStableId || ''), parentStableId: String(body?.parentStableId || ''),
        folderName: String(body?.folderName || ''),
      })
    } catch (error) { return accountFailure(error) }
  }
}
