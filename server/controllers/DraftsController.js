import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { parseDraftInput } from './validation.js'
import { ownedSenderAddresses, resolveOwnedSender } from '../../features/users/services/sender-addresses.js'

@RequestRoute("/drafts")
@Public
export class DraftsController {
  /** POST /drafts — create an idempotent durable draft. */
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
    const input = parseDraftInput(body)
    if (input instanceof Error) return Respond.r400(input.message)
    const app = await getApp()
    const addresses = await ownedSenderAddresses(app.users, auth.email)
    const sender = resolveOwnedSender(input.from, addresses, auth.email)
    if (!sender) return Respond.r403('From address is not owned by this mailbox')
    input.from = sender
    const result = await app.drafts.create(auth.email, input)
    return { ...result.draft, recovered: result.recovered }
  }
}
