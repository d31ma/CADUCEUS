import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { Actions } from '../platform/auth/Auth.js'
import { getApp } from '../app.js'
import { inspectExtensionBundle, presentExtension, trustedExtensionRecord } from '../features/extensions/services/extensions.js'

@RequestRoute("/extensions")
@SignedIn
export class ExtensionsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_READ } }, async ({ auth }) => {
      const app = await getApp()
      return (await app.extensions.listForOwner(auth.email))
        .sort((left, right) => left.displayName.localeCompare(right.displayName))
        .map(record => presentExtension(record))
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_WRITE } }, async ({ auth, body }) => {
      const inspected = inspectExtensionBundle(body?.bundle)
      if (inspected instanceof Error) return Respond.r400(inspected.message)
      if (typeof body?.trustedDigest !== 'string' || body.trustedDigest !== inspected.digest) return Respond.r400('The inspected extension digest must be explicitly trusted')
      const app = await getApp()
      if (await app.extensions.findForOwner(auth.email, inspected.manifest.id)) return Respond.r409('Extension is already installed')
      const record = trustedExtensionRecord(auth.email, inspected)
      await app.extensions.put(record)
      return presentExtension(record)
    })
  }
}
