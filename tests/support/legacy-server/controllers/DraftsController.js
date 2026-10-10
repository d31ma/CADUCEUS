import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { DraftInput } from '../shared/mailbox/DraftInput.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'

@RequestRoute("/drafts")
@SignedIn
export class DraftsController {
  /** @type {DraftService} */
  @Needs
  draftService

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /** POST /drafts — create an idempotent durable draft. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      const input = DraftInput.parseDraftInput(body)
      if (input instanceof Error) return Respond.r400(input.message)
      const app = this
      const addresses = await SenderAddresses.ownedSenderAddresses(app.usersHelper, auth.email)
      const sender = SenderAddresses.resolveOwnedSender(input.from, addresses, auth.email)
      if (!sender) return Respond.r403('From address is not owned by this mailbox')
      input.from = sender
      const result = await app.draftService.create(auth.email, input)
      return { ...result.draft, recovered: result.recovered }
    })
  }
}
