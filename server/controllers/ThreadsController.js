import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'
import { Respond } from '../shared/http/Respond.js'
import { SearchParser } from '../shared/mailbox/SearchParser.js'
import { presentExternalDetail } from '../shared/mailbox/external-message.js'
import {
  externalAttachmentSummaries,
  externalMailSummaries,
  isExternalAccountId,
  loadExternalMessage,
} from '../shared/mailbox/unified-mail.js'
import { groupMessagesIntoThreads } from '../features/mailbox/repositories/ThreadsRepository.js'
import { accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/threads")
@SignedIn
export class ThreadsController {
  static MAX_QUERY_LENGTH = 2_000

  static MAX_RESULTS = 1_000

  static MAX_EXTERNAL_CONVERSATION_SOURCE_BYTES = 25 * 1024 * 1024

  /** Load each store through its own owner boundary before forming any edges. */
  static async loadUnifiedThreadMessages(app, auth, token, allowedDomains, scope) {
    const [stored, external] = await Promise.all([
      scope.includeHosted ? app.emails.list(allowedDomains, { folder: 'all', ownerEmail: auth.email }) : Promise.resolve([]),
      scope.includeExternal ? externalMailSummaries(token, { accountId: scope.accountId }) : Promise.resolve([]),
    ])
    const migrated = await app.domainMigration.presentEmails(stored)
    const hosted = await app.categoryMappings.presentEmails(auth.email, migrated)
    return [...hosted, ...external]
  }

  /** Present one selected conversation, including bounded provider source data. */
  static async presentSelectedThread(app, token, thread, allowedDomains) {
    const attachments = await app.attachments.listSummariesByEmail(allowedDomains)
    let remainingSourceBytes = ThreadsController.MAX_EXTERNAL_CONVERSATION_SOURCE_BYTES
    const emails = []
    for (const email of thread.emails) {
      if (!email.external) {
        emails.push({ ...email, attachments: attachments.get(email.id) ?? [] })
        continue
      }
      const estimatedBytes = Math.floor(Number(email.rawSize || 0))
      if (estimatedBytes > remainingSourceBytes) {
        emails.push({
          ...email,
          body: email.snippet || '',
          attachments: externalAttachmentSummaries(email),
          conversationBodyTruncated: true,
        })
        continue
      }
      const loaded = await loadExternalMessage(token, email.id)
      if (!loaded) continue
      const sourceBytes = Math.floor(loaded.rawRFC822Base64.length * 0.75)
      if (sourceBytes > remainingSourceBytes) {
        emails.push({
          ...email,
          body: email.snippet || '',
          attachments: externalAttachmentSummaries(email),
          conversationBodyTruncated: true,
        })
        continue
      }
      const detail = await presentExternalDetail(loaded.indexed, loaded.rawRFC822Base64)
      if (!detail) continue
      remainingSourceBytes -= sourceBytes
      emails.push(detail)
    }
    return {
      ...thread,
      count: emails.length,
      emails,
      latestEmail: emails[emails.length - 1] || thread.latestEmail,
    }
  }

  static matchesFolder(email, requested, folderByDomain) {
    const wanted = String(requested || 'inbox').toLowerCase()
    const logical = String(email.systemFolder || email.folder || 'inbox').toLowerCase()
    if (logical === wanted) return true
    if (email.external) return false
    return String(email.folder || 'inbox').toLowerCase()
      === String(folderByDomain?.[email.domain] || wanted).toLowerCase()
  }

  /** @param {unknown} value */
  static readLimit(value) {
    if (value == null || value === '') return ThreadsController.MAX_RESULTS
    const limit = Number(value)
    if (!Number.isInteger(limit) || limit < 1 || limit > ThreadsController.MAX_RESULTS) {
      return new Error(`limit must be an integer from 1 to ${ThreadsController.MAX_RESULTS.toLocaleString('en-US')}`)
    }
    return limit
  }

  /**
 * GET /threads
 * @param {object} params
 * @param {{ folder?: string, emailId?: string, q?: string, domain?: string, account?: string, limit?: string|number }} params.query
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query, auth, context }) => {
      const app = await getApp()
      if (query?.q != null && typeof query.q !== 'string') return Respond.r400('q must be a string')
      const raw = String(query?.q || '').trim()
      if (raw.length > ThreadsController.MAX_QUERY_LENGTH) return Respond.r400('Search queries are limited to 2,000 characters')
      const domain = String(query?.domain || '').trim().toLowerCase()
      if (domain && !auth.domains.includes(domain)) return Respond.r403('Mailbox is outside the authenticated account scope')
      const accountId = String(query?.account || '').trim()
      if (accountId && !isExternalAccountId(accountId)) return Respond.r400('Invalid connected account identifier')
      if (domain && accountId) return Respond.r400('Choose only one mailbox domain or connected account')
      const limit = ThreadsController.readLimit(query?.limit)
      if (limit instanceof Error) return Respond.r400(limit.message)
      const allowedDomains = domain ? [domain] : auth.domains
      const token = bearerToken(context)

      let messages
      try {
        messages = await ThreadsController.loadUnifiedThreadMessages(app, auth, token, allowedDomains, {
          includeHosted: !accountId,
          includeExternal: !domain,
          accountId,
        })
      } catch (error) {
        return accountFailure(error)
      }
      const threads = groupMessagesIntoThreads(messages, auth.email)

      if (query?.emailId) {
        const thread = threads.find(candidate => candidate.emails.some(email => email.id === query.emailId))
        if (!thread) return []
        try {
          return [await ThreadsController.presentSelectedThread(app, token, thread, auth.domains)]
        } catch (error) {
          return accountFailure(error)
        }
      }

      if (raw) {
        const attachmentsByEmail = await app.attachments.listSummariesByEmail(allowedDomains)
        const expression = SearchParser.parseWithFallback(raw)
        const explicitCategory = SearchParser.hasField(expression, 'in')
        const matching = []
        for (const thread of threads) {
          const emails = thread.emails
          if (!emails.some(email => (
            (explicitCategory || !['trash', 'spam'].includes(
              String(email.systemFolder || email.folder || 'inbox').toLowerCase(),
            ))
            && SearchParser.matches(
              expression,
              email,
              email.external ? externalAttachmentSummaries(email) : attachmentsByEmail.get(email.id) || [],
            )
          ))) continue
          matching.push({
            ...thread,
            emails,
            latestEmail: emails[emails.length - 1],
          })
          if (matching.length >= limit) break
        }
        return matching
      }

      const folder = query?.folder || 'inbox'
      const folderByDomain = await app.categoryMappings.folderFilter(auth.email, allowedDomains, folder) || undefined
      return threads
        .filter(thread => thread.emails.some(email => ThreadsController.matchesFolder(email, folder, folderByDomain)))
        .slice(0, limit)
    })
  }
}
