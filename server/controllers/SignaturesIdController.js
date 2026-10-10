import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { Auth } from '../shared/auth/Auth.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'
import { SignatureContent } from '../shared/mailbox/SignatureContent.js'

@RequestRoute("/signatures/{id}")
@SignedIn
export class SignaturesIdController {
  /** @param {import('../features/mailbox/repositories/SignaturesRepository.js').SignatureEntry} entry */
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
      const app = await getApp()
      const [docId, existing] = await app.signatures.findById(id)
      if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Signature not found')
      const addresses = await SenderAddresses.ownedSenderAddresses(app.users, auth.email)
      const input = SignatureContent.normalizeSignatureInput(body, { partial: true, allowedAddresses: addresses })
      if (input instanceof Error) return Respond.r400(input.message)
      if (input.name) {
        input.name = SignatureContent.resolveUniqueSignatureName(input.name, Object.values(await app.signatures.listForOwner(auth.email)), id)
      }
      const patch = { ...input, updatedAt: Date.now() }
      if (input.defaultFor) await app.signatures.releaseDefaults(auth.email, input.defaultFor, id)
      await app.signatures.update(docId, patch)
      return SignaturesIdController.present({ ...existing, ...patch })
    })
  }

  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Signature not found')
      const app = await getApp()
      const [docId, existing] = await app.signatures.findById(id)
      if (!docId || !existing || existing.owner !== auth.email) return Respond.r404('Signature not found')
      await app.signatures.delete(docId)
      return { deleted: id }
    })
  }
}
