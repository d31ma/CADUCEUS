import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Auth } from '../shared/auth/Auth.js'
import { Extensions } from '../shared/extensions/Extensions.js'

@RequestRoute("/extensions/inspect")
@SignedIn
export class ExtensionsInspectController {
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ body }) => {
      const inspected = Extensions.inspectExtensionBundle(body?.bundle)
      if (inspected instanceof Error) return Respond.r400(inspected.message)
      return Extensions.presentExtension({ ...inspected, enabled: false })
    })
  }
}
