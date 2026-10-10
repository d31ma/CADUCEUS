import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { Actions } from '../platform/auth/Auth.js'
import { inspectExtensionBundle, presentExtension } from '../features/extensions/services/extensions.js'

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
