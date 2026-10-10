import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'

@RequestRoute("/presence/heartbeat")
@Public
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
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { auth }) {
  const app = await getApp()
  await app.presence.heartbeat(auth.email, auth.domains || [])
  return { ok: true }
}
}
