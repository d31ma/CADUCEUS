import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'
import { Respond } from '../shared/http/Respond.js'
import { SearchParser } from '../shared/mailbox/SearchParser.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'
import { UnifiedMail } from '../shared/mailbox/UnifiedMail.js'

@RequestRoute("/search")
@SignedIn
export class SearchController {
  static MAX_QUERY_LENGTH = 2_000

  static MAX_RESULTS = 1_000

  /** @param {unknown} value */
  static readLimit(value) {
    if (value == null || value === '') return SearchController.MAX_RESULTS
    const limit = Number(value)
    if (!Number.isInteger(limit) || limit < 1 || limit > SearchController.MAX_RESULTS) {
      return new Error(`limit must be an integer from 1 to ${SearchController.MAX_RESULTS.toLocaleString('en-US')}`)
    }
    return limit
  }

  /**
 * GET /search
 *
 * Full-text search with Gmail-like query syntax:
 *   `from:`, `to:`, `subject:`, `body:`, `attachment:`, `filename:`
 *   `has:attachment`, `is:unread`, `is:read`, `is:starred`
 *   `before:`, `after:` (ISO date strings)
 *
 * @param {object} params
 * @param {{ q?: string, domain?: string, account?: string, limit?: string|number }} params.query
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query, auth, context }) => {
      if (query?.q != null && typeof query.q !== 'string') return Respond.r400('q must be a string')
      const raw = String(query?.q || '').trim()
      if (raw.length > SearchController.MAX_QUERY_LENGTH) return Respond.r400('Search queries are limited to 2,000 characters')

      const domain = String(query?.domain || '').trim().toLowerCase()
      if (domain && !auth.domains.includes(domain)) return Respond.r403('Mailbox is outside the authenticated account scope')
      const accountId = String(query?.account || '').trim()
      if (accountId && !UnifiedMail.isExternalAccountId(accountId)) return Respond.r400('Invalid connected account identifier')
      if (domain && accountId) return Respond.r400('Choose only one mailbox domain or connected account')
      const limit = SearchController.readLimit(query?.limit)
      if (limit instanceof Error) return Respond.r400(limit.message)
      const allowedDomains = domain ? [domain] : auth.domains

      const app = await getApp()
      const [stored, attachmentsByEmail] = await Promise.all([
        accountId ? [] : app.emails.list(allowedDomains, { ownerEmail: auth.email }),
        app.attachments.listSummariesByEmail(allowedDomains),
      ])
      const migrated = await app.domainMigration.presentEmails(stored)
      const presented = await app.categoryMappings.presentEmails(auth.email, migrated)
      const expression = SearchParser.parseWithFallback(raw)
      let external
      try {
        external = domain ? [] : await UnifiedMail.externalMailSummaries(ExternalAccounts.bearerToken(context), { accountId })
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
      const hostedMatches = presented
        .filter(email => !raw || SearchParser.hasField(expression, 'in') || !['trash', 'spam'].includes(
          String(email.systemFolder || email.folder || 'inbox').toLowerCase(),
        ))
        .filter(email => SearchParser.matches(expression, email, attachmentsByEmail.get(email.id) || []))
      const externalMatches = external
        .filter(email => !raw || SearchParser.hasField(expression, 'in') || !['trash', 'spam'].includes(String(email.systemFolder || email.folder || 'inbox').toLowerCase()))
        .filter(email => SearchParser.matches(expression, email, UnifiedMail.externalAttachmentSummaries(email)))
      return UnifiedMail.mergeUnifiedMail(hostedMatches, externalMatches, { limit })
    })
  }
}
