import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { DraftInput } from '../shared/mailbox/DraftInput.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'

@RequestRoute("/drafts/{id}")
@SignedIn
export class DraftsIdController {
  /** @type {DraftService} */
  @Needs
  draftService

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Draft not found')
      const app = this
      return await app.draftService.get(id, auth.email, auth.domains) ?? Respond.r404('Draft not found')
    })
  }

  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Draft not found')
      if (!Number.isInteger(body?.version) || body.version < 1) return Respond.r400('version is required')
      const input = DraftInput.parseDraftInput(body)
      if (input instanceof Error) return Respond.r400(input.message)
      const app = this
      const addresses = await SenderAddresses.ownedSenderAddresses(app.usersHelper, auth.email)
      const sender = SenderAddresses.resolveOwnedSender(input.from, addresses, auth.email)
      if (!sender) return Respond.r403('From address is not owned by this mailbox')
      input.from = sender
      const result = await app.draftService.update(id, auth.email, auth.domains, body.version, input)
      if (result.kind === 'not-found') return Respond.r404('Draft not found')
      if (result.kind === 'conflict') {
        return {
          error: 'Draft changed in another session',
          conflict: true,
          currentVersion: result.currentVersion,
          updatedAt: result.updatedAt,
        }
      }
      return result.draft
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Draft not found')
      const app = this
      if (!await app.draftService.discard(id, auth.email, auth.domains)) return Respond.r404('Draft not found')
      return { discarded: id }
    })
  }
}
