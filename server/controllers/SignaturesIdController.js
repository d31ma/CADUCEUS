import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'
import { SignatureContent } from '../shared/mailbox/SignatureContent.js'

@RequestRoute("/signatures/{id}")
@SignedIn
export class SignaturesIdController {
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

  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, body, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Signature not found')
      const app = this
      const [docId, existing] = await app.signaturesHelper.findById(id)
      if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Signature not found')
      const addresses = await SenderAddresses.ownedSenderAddresses(app.usersHelper, auth.email)
      const input = SignatureContent.normalizeSignatureInput(body, { partial: true, allowedAddresses: addresses })
      if (input instanceof Error) return Respond.r400(input.message)
      if (input.name) {
        input.name = SignatureContent.resolveUniqueSignatureName(input.name, Object.values(await app.signaturesHelper.listForOwner(auth.email)), id)
      }
      const patch = { ...input, updatedAt: Date.now() }
      if (input.defaultFor) await app.signaturesHelper.releaseDefaults(auth.email, input.defaultFor, id)
      await app.signaturesHelper.update(docId, patch)
      return SignaturesIdController.present({ ...existing, ...patch })
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Signature not found')
      const app = this
      const [docId, existing] = await app.signaturesHelper.findById(id)
      if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Signature not found')
      await app.signaturesHelper.delete(docId)
      return { deleted: id }
    })
  }
}
