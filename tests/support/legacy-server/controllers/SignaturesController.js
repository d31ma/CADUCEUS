import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'
import { SignatureContent } from '../shared/mailbox/SignatureContent.js'

@RequestRoute("/signatures")
@SignedIn
export class SignaturesController {
  /** @type {SignaturesHelper} */
  @Needs
  signaturesHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /** @param {import('../helpers/SignaturesHelper.js').SignatureEntry} entry */
  static present(entry) {
    return {
      id: entry.id,
      domain: entry.domain,
      name: entry.name,
      text: entry.text,
      defaultFor: entry.defaultFor ?? [],
      createdAt: new Date(entry.createdAt).toISOString(),
      updatedAt: new Date(entry.updatedAt || entry.createdAt).toISOString(),
    }
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
      const app = this
      const addresses = await SenderAddresses.ownedSenderAddresses(app.usersHelper, auth.email)
      const input = SignatureContent.normalizeSignatureInput(body, { allowedAddresses: addresses })
      if (input instanceof Error) return Respond.r400(input.message)
      const existing = Object.values(await app.signaturesHelper.listForOwner(auth.email))
      const now = Date.now()
      const entry = {
        id: `sig-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
        owner: auth.email,
        domain: auth.email.split('@')[1] || auth.domain,
        name: SignatureContent.resolveUniqueSignatureName(input.name ?? '', existing),
        text: input.text ?? '',
        defaultFor: input.defaultFor ?? [],
        createdAt: now,
        updatedAt: now,
      }
      await app.signaturesHelper.releaseDefaults(auth.email, entry.defaultFor, entry.id)
      await app.signaturesHelper.put(entry)
      return SignaturesController.present(entry)
    })
  }

  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ auth }) => {
      const app = this
      const records = await app.signaturesHelper.listForOwner(auth.email)
      return Object.values(records).sort((left, right) => left.name.localeCompare(right.name)).map(SignaturesController.present)
    })
  }
}
