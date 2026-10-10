// @ts-check

import { Fylo } from '../shared/persistence/Fylo.js'
import { PresenceRules } from '../shared/presence/PresenceRules.js'

@Service
export default class PresenceService {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * Mark a user as currently online.
   *
   * @param {string} email
   * @param {string[]} domains
   */
  async heartbeat(email, domains) {
    const fylo = await this.fyloHelper.database()
    if (!email) return
    const norm = email.toLowerCase()
    const [prevId] = await this._findByEmail(norm)
    if (prevId) await fylo.delDoc(Fylo.Collections.PRESENCE, prevId)
    await fylo.putData(Fylo.Collections.PRESENCE, {
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
    const fylo = await this.fyloHelper.database()
    if (!Array.isArray(viewerDomains) || viewerDomains.length === 0) return []
    const viewerSet = new Set(viewerDomains.map(d => d.toLowerCase()))
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.PRESENCE, { $ops: [] }).collect())
    const now = Date.now()

    /** @type {Array<{ email: string, online: boolean, lastSeenAt: string }>} */
    const out = []
    for (const [docId, rec] of Object.entries(docs)) {
      const r = /** @type {PresenceRecord} */ (rec)
      if (now - r.lastSeenAt > PresenceRules.PRUNE_AFTER_MS) {
        fylo.delDoc(Fylo.Collections.PRESENCE, docId).catch(() => {})
        continue
      }
      if (!(r.domains || []).some(d => viewerSet.has(d.toLowerCase()))) continue
      out.push({
        email: r.email,
        online: now - r.lastSeenAt < PresenceRules.ONLINE_WINDOW_MS,
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
    return !!rec && Date.now() - rec.lastSeenAt < PresenceRules.ONLINE_WINDOW_MS
  }

  /**
   * @param {string} email
   * @returns {Promise<[string | null, PresenceRecord | null]>}
   */
  async _findByEmail(email) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(
      fylo.findDocs(Fylo.Collections.PRESENCE, { $ops: [{ email: { $eq: email } }] }).collect(),
    )
    const entry = Object.entries(docs)[0]
    if (!entry) return [null, null]
    return [entry[0], /** @type {PresenceRecord} */ (entry[1])]
  }
}
