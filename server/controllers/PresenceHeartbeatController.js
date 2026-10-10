import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'

@RequestRoute("/presence/heartbeat")
@SignedIn
export class PresenceHeartbeatController {
  /**
 * POST /presence/heartbeat
 *
 * Mark the caller as currently active. Clients ping this on a 30s timer and
 * on tab refocus so their presence TTL doesn't expire while they're using
 * the app.
 *
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = await getApp()
    await app.presence.heartbeat(auth.email, auth.domains || [])
    return { ok: true }
    })
  }
}
