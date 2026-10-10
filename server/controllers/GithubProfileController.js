import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Security } from '../shared/security/Security.js'
import { getApp } from '../app.js'

@RequestRoute("/github/profile")
@SignedIn
export class GithubProfileController {
  /** @param {Record<string, any>} message */
  static participantAddresses(message) {
    const values = [
      message.sender,
      message.recipient,
      message.originalRecipient,
      ...(Array.isArray(message.to) ? message.to : []),
      ...(Array.isArray(message.cc) ? message.cc : []),
      ...(Array.isArray(message.bcc) ? message.bcc : []),
    ]
    return [...new Set(values.flatMap(value => String(value || '').match(GithubProfileController.ADDRESS_PATTERN) || []).map(value => value.toLowerCase()))]
  }

  static ADDRESS_PATTERN = /[A-Z0-9.!#$%&\x27*+/=?^_\x60{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+/gi

  /**
   * GET /github/profile?email=person@example.com
   *
   * Best-effort public GitHub enrichment is disabled by default. Even after
   * opt-in, a caller may query only an exact personal contact or participant
   * in mail owned by that same mailbox.
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query, context, auth }) => {
      const params = query || context?.query || {}
      const email = Security.normalizeEmailAddress(params.email)
      if (!email) return Respond.r400('email must be a valid email address')

      const app = await getApp()
      const [, user] = await app.users.findByEmail(auth.email)
      if (!user) return Respond.r404('User not found')
      if (user.githubProfileEnrichment !== true) return { enabled: false, status: 'disabled' }

      const contact = await app.contactRecords.findByEmail(auth.email, email)
      const related = contact || await GithubProfileController.hasOwnedRelationship(app, auth, email)
      if (!related) return Respond.r404('Contact not found')

      const result = await app.githubProfiles.findByEmail(email)
      return { enabled: true, ...result }
    })
  }

  /**
   * Use the exact-owner repository boundary first, then verify an exact
   * address field so a body-text substring can never authorize enrichment.
   */
  static async hasOwnedRelationship(app, auth, email) {
    const candidates = await app.emails.list(auth.domains, {
      ownerEmail: auth.email,
      query: email,
      limit: 20,
    })
    return candidates.some(message => GithubProfileController.participantAddresses(message).includes(email))
  }
}
