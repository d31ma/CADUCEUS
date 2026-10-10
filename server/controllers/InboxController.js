import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'
import { externalMailSummaries, isExternalAccountId, mergeUnifiedMail } from '../features/mailbox/services/unified-mail.js'

@RequestRoute("/inbox")
@SignedIn
export class InboxController {
  /**
   * GET /inbox
   * @param {object} params
   * @param {{ q?: string, search?: string, folder?: string, domain?: string, account?: string, read?: boolean, starred?: boolean, hasAttachment?: boolean, limit?: number, offset?: number }} params.query
   * @param {{ bearer?: { token: string } }} params.context
   * @returns {Promise<unknown>}
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query, auth, context }) => {
      const filters = InboxController._parseFilters(query ?? {})
      if (filters instanceof Error) return Respond.r400(filters.message)
      if (filters.domain && filters.accountId) return Respond.r400('Choose only one mailbox domain or connected account')
      if (filters.domain && !auth.domains.includes(filters.domain)) {
        return Respond.r403('Mailbox is outside the authenticated account scope')
      }

      const app = await getApp()
      // Processing is idempotent and scoped to the authenticated mailbox. This
      // guarantees reminders resurface even after a server restart or a period
      // with no browser connected, without relying on an in-memory timer.
      await app.reminders.processDue(auth.email)
      filters.ownerEmail = auth.email
      const allowedDomains = filters.domain ? [filters.domain] : auth.domains
      if (filters.folder && !filters.accountId) {
        filters.folderByDomain = await app.categoryMappings.folderFilter(auth.email, allowedDomains, filters.folder) || undefined
      }
      const offset = filters.offset
      const limit = filters.limit
      const hostedFilters = { ...filters }
      delete hostedFilters.offset
      delete hostedFilters.limit
      delete hostedFilters.domain
      delete hostedFilters.accountId
      const emails = filters.accountId ? [] : await app.emails.list(allowedDomains, hostedFilters)
      const presented = await app.domainMigration.presentEmails(emails)
      const hosted = await app.categoryMappings.presentEmails(auth.email, presented)
      try {
        let external = filters.domain ? [] : await externalMailSummaries(bearerToken(context), {
          accountId: filters.accountId,
          folderRole: filters.folder,
          query: filters.query,
          read: filters.read,
          starred: filters.starred,
        })
        if (typeof filters.hasAttachment === 'boolean') {
          external = external.filter(message => message.hasAttachments === filters.hasAttachment)
        }
        return mergeUnifiedMail(hosted, external, { offset, limit })
      } catch (error) {
        return accountFailure(error)
      }
    })
  }

  /**
   * @param {Record<string, any>} query
   * @returns {Record<string, any> | Error}
   */
  static _parseFilters(query) {
    /** @type {Record<string, any>} */
    const filters = {}

    const q = InboxController._readString(query.q ?? query.search)
    if (q) filters.query = q

    const folder = InboxController._readString(query.folder)
    if (folder) filters.folder = folder

    const domain = InboxController._readString(query.domain)?.toLowerCase()
    if (domain) filters.domain = domain

    const accountId = InboxController._readString(query.account)
    if (accountId && !isExternalAccountId(accountId)) return new Error('Invalid connected account identifier')
    if (accountId) filters.accountId = accountId

    const read = InboxController._readBoolean(query.read)
    if (read instanceof Error) return read
    if (read != null) filters.read = read

    const starred = InboxController._readBoolean(query.starred)
    if (starred instanceof Error) return starred
    if (starred != null) filters.starred = starred

    const hasAttachment = InboxController._readBoolean(query.hasAttachment ?? query.attachments)
    if (hasAttachment instanceof Error) return hasAttachment
    if (hasAttachment != null) filters.hasAttachment = hasAttachment

    const limit = InboxController._readNumber(query.limit, 'limit')
    if (limit instanceof Error) return limit
    if (limit != null) filters.limit = limit

    const offset = InboxController._readNumber(query.offset, 'offset')
    if (offset instanceof Error) return offset
    if (offset != null) filters.offset = offset

    return filters
  }

  /** @param {unknown} value @returns {string | undefined} */
  static _readString(value) {
    return typeof value === 'string' && value.trim() ? value.trim() : undefined
  }

  /** @param {unknown} value @returns {boolean | undefined | Error} */
  static _readBoolean(value) {
    if (value == null || value === '') return undefined
    if (value === true || value === 'true' || value === '1') return true
    if (value === false || value === 'false' || value === '0') return false
    return new Error('Invalid boolean query parameter')
  }

  /** @param {unknown} value @param {string} name @returns {number | undefined | Error} */
  static _readNumber(value, name) {
    if (value == null || value === '') return undefined
    const parsed = Number(value)
    if (!Number.isInteger(parsed) || parsed < 0) return new Error(`Invalid ${name} query parameter`)
    return parsed
  }
}
