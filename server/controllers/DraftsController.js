import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { DraftInput } from '../shared/mailbox/DraftInput.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'

@RequestRoute("/drafts")
@SignedIn
export class DraftsController {
  /** POST /drafts — create an idempotent durable draft. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      const input = DraftInput.parseDraftInput(body)
      if (input instanceof Error) return Respond.r400(input.message)
      const app = await getApp()
      const addresses = await SenderAddresses.ownedSenderAddresses(app.users, auth.email)
      const sender = SenderAddresses.resolveOwnedSender(input.from, addresses, auth.email)
      if (!sender) return Respond.r403('From address is not owned by this mailbox')
      input.from = sender
      const result = await app.drafts.create(auth.email, input)
      return { ...result.draft, recovered: result.recovered }
    })
  }
}
