import { AuthGuard } from '../../../middleware/require-auth.js'
import { getApp } from '../../../app.js'
import { FyloAccessContext } from '../../../platform/persistence/fylo-access.js'
import { externalMailSummaries } from '../../../features/mailbox/services/unified-mail.js'
import { Buffer } from 'node:buffer'

const STREAM_MAX_MS = Number(process.env.STREAM_MAX_MS ?? 25_000)
const STREAM_POLL_MS = Number(process.env.STREAM_POLL_MS ?? 2_000)
const REPLAY_LIMIT = Number(process.env.STREAM_REPLAY_LIMIT ?? 50)

/**
 * GET /inbox/stream — live new-mail via Server-Sent Events.
 *
 * Tachyon streams this when the request carries `Accept: text/event-stream`:
 * the handler RETURNS an async generator (it is not itself a generator, so the
 * inline auth check can short-circuit with a normal JSON 401 before any
 * streaming begins).
 *
 * Bounded, cursor-based design:
 *   - The client passes an opaque two-source cursor. Legacy ISO timestamps are
 *     accepted and promoted to independent hosted/provider positions.
 *   - On (re)connect we immediately replay the oldest bounded slice newer than
 *     each position, so advancing one source never skips a gap in the other.
 *   - We then poll for new mail on a short interval, emitting each as an SSE
 *     `new-email` event whose `id:` is the next opaque composite cursor.
 *   - The connection periodically self-closes after `STREAM_MAX_MS` (25s by
 *     default) with a `reconnect` event; the client reconnects with its latest
 *     cursor. Operators can raise the value for longer-lived connections.
 *
 * Web Push (PushService, already fired from inbound delivery) covers the case
 * where the client is fully offline/backgrounded.
 */
@RequestRoute("/inbox/stream")
@Public
export class InboxStreamController {
  /**
   * @param {object} params
   * @param {{ since?: string, token?: string }} params.query
   * @param {{ bearer?: { token: string } }} params.context
   */
  @GetRoute
  async get({ query, context }) {
    // EventSource can't set Authorization headers; allow `?token=` as a
    // fallback to the bearer header so either client style authenticates.
    const ctx = context?.bearer?.token
      ? context
      : { bearer: { token: typeof query?.token === 'string' ? query.token : '' } }
    const auth = await AuthGuard.requireAuth(ctx)
    // Tachyon commits to a 200 stream once Accept: text/event-stream is seen,
    // so a JSON 401 can't be returned mid-stream. Emit an `unauthorized` SSE
    // event instead; the client stops reconnecting and signs out.
    return auth.ok ? InboxStreamController._streamInbox(auth.principal, query ?? {}, ctx.bearer?.token || '') : InboxStreamController._unauthorizedStream()
  }

  static async *_unauthorizedStream() {
    yield InboxStreamController._sse('unauthorized', { error: 'Authentication required' }, null)
  }

  /**
   * @param {import('../../../middleware/require-auth.js').Principal} claims
   * @param {{ since?: string }} query
   * @param {string} token
   */
  static async *_streamInbox(claims, query, token) {
    const app = await getApp()
    const domains = [claims.domain]
    const cursor = InboxStreamController._parseCursor(typeof query.since === 'string' && query.since ? query.since : new Date().toISOString())

    // Advise the browser's reconnect backoff and announce the live cursor.
    yield `retry: ${STREAM_POLL_MS}\n`
    yield InboxStreamController._sse('connected', { since: InboxStreamController._cursorID(cursor) }, null)

    const deadline = Date.now() + STREAM_MAX_MS
    try {
      while (Date.now() < deadline) {
        const hosted = await FyloAccessContext.run(
          { email: claims.address, domains },
          () => InboxStreamController._freshHostedSince(app, domains, cursor.hosted, claims.address),
        )
        let external = []
        try {
          external = await InboxStreamController._freshExternalSince(token, cursor.external)
        } catch {
          // Keep the provider cursor unchanged. A later reconnect replays the
          // gap even if hosted mail advances independently.
        }
        const fresh = [...hosted, ...external].sort((left, right) => {
          const time = String(left.receivedAt || '').localeCompare(String(right.receivedAt || ''))
          return time || String(left.id || '').localeCompare(String(right.id || ''))
        }).slice(0, REPLAY_LIMIT)
        for (const email of fresh) {
          const source = email.external === true ? cursor.external : cursor.hosted
          source.at = String(email.receivedAt || source.at)
          source.id = String(email.id || '')
          yield InboxStreamController._sse('new-email', InboxStreamController._summarize(email), InboxStreamController._cursorID(cursor))
        }
        if (fresh.length === 0) yield `: ping\n\n` // heartbeat keeps the pipe warm
        await InboxStreamController._sleep(STREAM_POLL_MS)
      }
    } catch (err) {
      yield InboxStreamController._sse('error', { message: err instanceof Error ? err.message : 'stream error' }, null)
    }
    // Ask the client to reconnect with its now-current cursor.
    yield InboxStreamController._sse('reconnect', { since: InboxStreamController._cursorID(cursor) }, null)
  }

  /**
   * Inbox emails newer than the cursor, oldest-first so the client appends in
   * order. Capped so a long-offline reconnect can't flood the pipe.
   *
   * @param {import('../../../app.js').App} app
   * @param {string[]} domains
   * @param {string} cursor
   * @param {string} ownerEmail
   */
  static async _freshSince(app, domains, cursor, ownerEmail) {
    return InboxStreamController._freshHostedSince(app, domains, InboxStreamController._parseSourceCursor(cursor), ownerEmail)
  }

  static async _freshHostedSince(app, domains, cursor, ownerEmail) {
    const folderByDomain = await app.categoryMappings.folderFilter(ownerEmail, domains, 'inbox') || undefined
    const since = InboxStreamController._replaySince(cursor.at)
    const emails = await app.emails.list(domains, {
      folder: 'inbox',
      folderByDomain,
      since,
      ownerEmail,
    })
    return (await app.categoryMappings.presentEmails(ownerEmail, emails))
      .filter(email => InboxStreamController._afterCursor(email, cursor))
      .slice().reverse()
      .slice(0, REPLAY_LIMIT)
  }

  static async _freshExternalSince(token, cursor) {
    const result = []
    let offset = 0
    while (result.length < REPLAY_LIMIT && offset <= 100_000) {
      const page = await externalMailSummaries(token, {
        folderRole: 'inbox', receivedAtFrom: cursor.at, oldestFirst: true, limit: 1000, offset,
      })
      result.push(...page.filter(email => InboxStreamController._afterCursor(email, cursor)))
      if (page.length < 1000) break
      offset += page.length
    }
    return result.slice(0, REPLAY_LIMIT)
  }

  /** Lightweight row payload — never stream full bodies/attachments. */
  static _summarize(e) {
    return {
      id: e.id,
      domain: e.domain,
      sender: e.sender,
      recipient: e.recipient,
      subject: e.subject,
      folder: e.folder,
      systemFolder: e.systemFolder,
      read: e.read,
      starred: e.starred,
      receivedAt: e.receivedAt,
      ...(e.external === true ? {
        external: true,
        providerAccountId: e.providerAccountId,
        providerStableId: e.providerStableId,
        mailboxAddress: e.mailboxAddress,
        mailboxLabel: e.mailboxLabel,
        mailboxColor: e.mailboxColor,
      } : {}),
      ...(e.spamScore != null ? { spamScore: e.spamScore } : {}),
    }
  }

  /**
   * Format one Server-Sent Event frame.
   * @param {string} event @param {unknown} data @param {string | null} id
   */
  static _sse(event, data, id) {
    let frame = ''
    if (id != null) frame += `id: ${id}\n`
    if (event) frame += `event: ${event}\n`
    frame += `data: ${JSON.stringify(data)}\n\n`
    return frame
  }

  static _parseSourceCursor(value) {
    const at = typeof value === 'string' && Number.isFinite(Date.parse(value)) ? new Date(value).toISOString() : new Date().toISOString()
    return { at, id: '' }
  }

  static _parseCursor(value) {
    if (typeof value === 'string' && value.startsWith('v1.')) {
      try {
        const parsed = JSON.parse(Buffer.from(value.slice(3), 'base64url').toString('utf8'))
        if (parsed && typeof parsed === 'object') {
          return {
            hosted: InboxStreamController._validSourceCursor(parsed.hosted),
            external: InboxStreamController._validSourceCursor(parsed.external),
          }
        }
      } catch {}
    }
    const source = InboxStreamController._parseSourceCursor(value)
    return { hosted: { ...source }, external: { ...source } }
  }

  static _validSourceCursor(value) {
    if (!value || typeof value !== 'object' || typeof value.at !== 'string' || !Number.isFinite(Date.parse(value.at)) || typeof value.id !== 'string' || value.id.length > 2048) {
      return InboxStreamController._parseSourceCursor('')
    }
    return { at: new Date(value.at).toISOString(), id: value.id }
  }

  static _cursorID(cursor) {
    return 'v1.' + Buffer.from(JSON.stringify(cursor), 'utf8').toString('base64url')
  }

  static _replaySince(value) {
    const parsed = Date.parse(value)
    return Number.isFinite(parsed) ? new Date(parsed - 1).toISOString() : value
  }

  static _afterCursor(email, cursor) {
    const at = String(email?.receivedAt || '')
    return at > cursor.at || at === cursor.at && String(email?.id || '') > cursor.id
  }

  /** @param {number} ms */
  static _sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms))
  }
}
