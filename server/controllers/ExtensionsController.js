import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { getApp } from '../app.js'
import { Extensions } from '../shared/extensions/Extensions.js'

@RequestRoute("/extensions")
@SignedIn
export class ExtensionsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ auth }) => {
      const app = await getApp()
      return (await app.extensions.listForOwner(auth.email))
        .sort((left, right) => left.displayName.localeCompare(right.displayName))
        .map(record => Extensions.presentExtension(record))
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ auth, body }) => {
      const inspected = Extensions.inspectExtensionBundle(body?.bundle)
      if (inspected instanceof Error) return Respond.r400(inspected.message)
      if (typeof body?.trustedDigest !== 'string' || body.trustedDigest !== inspected.digest) return Respond.r400('The inspected extension digest must be explicitly trusted')
      const app = await getApp()
      if (await app.extensions.findForOwner(auth.email, inspected.manifest.id)) return Respond.r409('Extension is already installed')
      const record = Extensions.trustedExtensionRecord(auth.email, inspected)
      await app.extensions.put(record)
      return Extensions.presentExtension(record)
    })
  }
}
