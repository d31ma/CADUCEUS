import { Guards } from '../../middleware/guards.js'
import { getApp } from '../../app.js'
import { Respond } from '../../platform/http/respond.js'
import { SearchParser } from '../../features/mailbox/services/search-parser.js'
import { accountFailure, bearerToken } from '../../platform/accounts/bridge.js'
import { externalAttachmentSummaries, externalMailSummaries, isExternalAccountId, mergeUnifiedMail } from '../../features/mailbox/services/unified-mail.js'

const MAX_QUERY_LENGTH = 2_000
const MAX_RESULTS = 1_000

@RequestRoute("/search")
@Public
export class SearchController {
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
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { query, auth, context }) {
    if (query?.q != null && typeof query.q !== 'string') return Respond.r400('q must be a string')
    const raw = String(query?.q || '').trim()
    if (raw.length > MAX_QUERY_LENGTH) return Respond.r400('Search queries are limited to 2,000 characters')

    const domain = String(query?.domain || '').trim().toLowerCase()
    if (domain && !auth.domains.includes(domain)) return Respond.r403('Mailbox is outside the authenticated account scope')
    const accountId = String(query?.account || '').trim()
    if (accountId && !isExternalAccountId(accountId)) return Respond.r400('Invalid connected account identifier')
    if (domain && accountId) return Respond.r400('Choose only one mailbox domain or connected account')
    const limit = readLimit(query?.limit)
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
      external = domain ? [] : await externalMailSummaries(bearerToken(context), { accountId })
    } catch (error) {
      return accountFailure(error)
    }
    const hostedMatches = presented
      .filter(email => !raw || SearchParser.hasField(expression, 'in') || !['trash', 'spam'].includes(
        String(email.systemFolder || email.folder || 'inbox').toLowerCase(),
      ))
      .filter(email => SearchParser.matches(expression, email, attachmentsByEmail.get(email.id) || []))
    const externalMatches = external
      .filter(email => !raw || SearchParser.hasField(expression, 'in') || !['trash', 'spam'].includes(String(email.systemFolder || email.folder || 'inbox').toLowerCase()))
      .filter(email => SearchParser.matches(expression, email, externalAttachmentSummaries(email)))
    return mergeUnifiedMail(hostedMatches, externalMatches, { limit })
  }
}

/** @param {unknown} value */
function readLimit(value) {
  if (value == null || value === '') return MAX_RESULTS
  const limit = Number(value)
  if (!Number.isInteger(limit) || limit < 1 || limit > MAX_RESULTS) {
    return new Error(`limit must be an integer from 1 to ${MAX_RESULTS.toLocaleString('en-US')}`)
  }
  return limit
}
