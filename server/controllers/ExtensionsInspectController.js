import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Actions } from '../shared/auth/Auth.js'
import { inspectExtensionBundle, presentExtension } from '../shared/extensions/extensions.js'

@RequestRoute("/extensions/inspect")
@SignedIn
export class ExtensionsInspectController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_READ } }, async ({ body }) => {
      const inspected = inspectExtensionBundle(body?.bundle)
      if (inspected instanceof Error) return Respond.r400(inspected.message)
      return presentExtension({ ...inspected, enabled: false })
    })
  }
}
