import { RouteRequest } from '../shared/http/RouteRequest.js'

@RequestRoute("/presence/team")
@SignedIn
export class PresenceTeamController {
  /** @type {PresenceService} */
  @Needs
  presenceService

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /**
 * GET /presence/team
 *
 * Return every user the caller can see in their domain(s), with online status
 * from the presence map. We surface the full user list (not just users with
 * recent heartbeats) so the UI can render "offline" members too — useful for
 * an at-a-glance team view.
 *
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const viewerDomains = (auth.domains || []).map(d => d.toLowerCase())

    const app = this
    const allUsers = await app.usersHelper.list()

    // De-duplicate by email — the dev seed historically produced multiple user
    // records per email, which would render twice in the Team roster.
    const seenEmails = new Set()
    const uniqueUsers = []
    for (const u of allUsers) {
      const k = (u.email || '').toLowerCase()
      if (!k || seenEmails.has(k)) continue
      seenEmails.add(k)
      uniqueUsers.push(u)
    }

    // Same-domain teammates only — never leak users from unrelated domains.
    const teammates = uniqueUsers.filter(u => {
      return (u.domains || []).map(d => d.toLowerCase()).some(d => viewerDomains.includes(d))
    })

    const presenceList = await app.presenceService.snapshotForDomains(viewerDomains)
    const presenceByEmail = new Map(presenceList.map(p => [p.email.toLowerCase(), p]))

    return teammates
      .map(u => {
        const p = presenceByEmail.get((u.email || '').toLowerCase())
        return {
          email: u.email,
          name: /** @type {any} */ (u).name || '',
          role: u.role || '',
          domains: u.domains || [],
          online: !!p?.online,
          lastSeenAt: p?.lastSeenAt || null,
          isSelf: (u.email || '').toLowerCase() === (auth.email || '').toLowerCase(),
        }
      })
      .sort((a, b) => {
        // Online users first, then alphabetical by email.
        if (a.online !== b.online) return a.online ? -1 : 1
        return a.email.localeCompare(b.email)
      })
    })
  }
}
