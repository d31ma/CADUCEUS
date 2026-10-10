import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { ownedSenderAddresses } from '../../features/users/services/sender-addresses.js'

const DAY_MS = 24 * 60 * 60 * 1000
const RANGES = { '7d': 7, '30d': 30, '90d': 90 }

@RequestRoute("/activity")
@Public
export class ActivityController {
  /**
   * GET /activity
   *
   * @param {{ query?: Record<string, string>, auth: { email: string, domains: string[] } }} params
   */
  @Guards.auth()
  @GetRoute
  async get({ query = {}, auth }) {
    const parsed = parseActivityQuery(query)
    if (!parsed.ok) return Respond.r400(parsed.error)
    const app = await getApp()
    const senders = await ownedSenderAddresses(app.users, auth.email)
    const sentFolders = await app.categoryMappings.folderFilter(auth.email, auth.domains || [], 'sent') || {}
    const draftFolders = await app.categoryMappings.folderFilter(auth.email, auth.domains || [], 'drafts') || {}
    return await app.tracking.activity({
      allowedDomains: auth.domains || [],
      ownerEmail: auth.email,
      ownedSenders: senders,
      sentFolders,
      draftFolders,
      ...parsed.value,
    })
  }
}

/**
 * @param {Record<string, string>} query
 * @returns {{ ok: true, value: { type: 'all'|'open'|'click', from: number, to: number, limit: number, since?: number } } | { ok: false, error: string }}
 */
export function parseActivityQuery(query) {
  const requestedType = query.type || 'all'
  if (!['all', 'open', 'click'].includes(requestedType)) return { ok: false, error: 'type must be all, open, or click' }
  const type = /** @type {'all'|'open'|'click'} */ (requestedType)
  const requestedRange = query.range || '30d'
  if (![...Object.keys(RANGES), 'all'].includes(requestedRange)) return { ok: false, error: 'range must be 7d, 30d, 90d, or all' }
  const range = /** @type {'7d'|'30d'|'90d'|'all'} */ (requestedRange)
  const now = Date.now()
  let from = range === 'all' ? 0 : now - RANGES[range] * DAY_MS
  let to = now
  if (query.from) {
    from = Date.parse(query.from)
    if (!Number.isFinite(from)) return { ok: false, error: 'from must be an ISO datetime' }
  }
  if (query.to) {
    to = Date.parse(query.to)
    if (!Number.isFinite(to)) return { ok: false, error: 'to must be an ISO datetime' }
  }
  if (from > to) return { ok: false, error: 'from must not be after to' }
  /** @type {number | undefined} */
  let since
  if (query.since) {
    since = Date.parse(query.since)
    if (!Number.isFinite(since)) return { ok: false, error: 'since must be an ISO datetime' }
  }
  const limit = query.limit == null || query.limit === '' ? 50 : Number(query.limit)
  if (!Number.isInteger(limit) || limit < 1 || limit > 100) return { ok: false, error: 'limit must be an integer from 1 to 100' }
  return {
    ok: true,
    value: {
      type,
      from,
      to,
      limit,
      ...(Number.isFinite(since) ? { since } : {}),
    },
  }
}
