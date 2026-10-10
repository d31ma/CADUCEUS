// @ts-check

import { Fylo } from '../../../shared/persistence/Fylo.js'

/**
 * Presence tracking — who's online right now, scoped per domain.
 *
 * Persisted in Fylo under {@link Collections.PRESENCE} so heartbeats from any
 * Tachyon worker share the same view (Tachyon spawns a fresh subprocess per
 * request, so a pure in-memory map can't synchronise across endpoints).
 *
 * One document per user — the `email` field is the primary key. Updates are
 * delete-then-put because Fylo's `putData` always inserts under a fresh
 * docId. A user is "online" if `lastSeenAt` is within ONLINE_WINDOW_MS.
 * Records older than PRUNE_AFTER_MS are pruned lazily on snapshot reads.
 *
 * @module presence
 */

const ONLINE_WINDOW_MS = 60_000
const PRUNE_AFTER_MS = 10 * 60_000

export class PresenceService {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * Mark a user as currently online.
   *
   * @param {string} email
   * @param {string[]} domains
   */
  async heartbeat(email, domains) {
    if (!email) return
    const norm = email.toLowerCase()
    const [prevId] = await this._findByEmail(norm)
    if (prevId) await this.fylo.delDoc(Fylo.Collections.PRESENCE, prevId)
    await this.fylo.putData(Fylo.Collections.PRESENCE, {
      email: norm,
      domains: Array.isArray(domains) ? domains : [],
      lastSeenAt: Date.now(),
    })
  }

  /**
   * Snapshot every user we've heard from recently that shares at least one
   * domain with the caller. Lazy-prunes records older than {@link PRUNE_AFTER_MS}.
   *
   * @param {string[]} viewerDomains
   * @returns {Promise<Array<{ email: string, online: boolean, lastSeenAt: string }>>}
   */
  async snapshotForDomains(viewerDomains) {
    if (!Array.isArray(viewerDomains) || viewerDomains.length === 0) return []
    const viewerSet = new Set(viewerDomains.map(d => d.toLowerCase()))
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.PRESENCE, { $ops: [] }).collect())
    const now = Date.now()

    /** @type {Array<{ email: string, online: boolean, lastSeenAt: string }>} */
    const out = []
    for (const [docId, rec] of Object.entries(docs)) {
      const r = /** @type {PresenceRecord} */ (rec)
      if (now - r.lastSeenAt > PRUNE_AFTER_MS) {
        this.fylo.delDoc(Fylo.Collections.PRESENCE, docId).catch(() => {})
        continue
      }
      if (!(r.domains || []).some(d => viewerSet.has(d.toLowerCase()))) continue
      out.push({
        email: r.email,
        online: now - r.lastSeenAt < ONLINE_WINDOW_MS,
        lastSeenAt: new Date(r.lastSeenAt).toISOString(),
      })
    }
    return out
  }

  /**
   * @param {string} email
   * @returns {Promise<boolean>}
   */
  async isOnline(email) {
    const [, rec] = await this._findByEmail((email || '').toLowerCase())
    return !!rec && Date.now() - rec.lastSeenAt < ONLINE_WINDOW_MS
  }

  /**
   * @param {string} email
   * @returns {Promise<[string | null, PresenceRecord | null]>}
   */
  async _findByEmail(email) {
    const docs = await Fylo.collect(
      this.fylo.findDocs(Fylo.Collections.PRESENCE, { $ops: [{ email: { $eq: email } }] }).collect(),
    )
    const entry = Object.entries(docs)[0]
    if (!entry) return [null, null]
    return [entry[0], /** @type {PresenceRecord} */ (entry[1])]
  }
}
