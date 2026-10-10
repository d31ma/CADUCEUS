import { Fylo } from '../shared/persistence/Fylo.js'
import { Security } from '../shared/security/Security.js'
import { RateLimitRules } from '../shared/rate-limit/RateLimitRules.js'

/**
 * Fylo-backed rate limiter shared by the self-hosted application processes.
 */
@Service
export default class RateLimitService {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @param {string | string[]} key
   * @param {number} maxRequests
   * @param {number} windowMs
   * @returns {Promise<RateLimitResult>}
   */
  async check(key, maxRequests, windowMs) {
    const windowSeconds = Math.ceil(windowMs / 1000)
    return this._checkFylo(key, maxRequests, windowSeconds)
  }

  /**
   * @param {string | string[]} key
   * @param {number} maxRequests
   * @param {number} windowSeconds
   * @returns {Promise<RateLimitResult>}
   */
  async _checkFylo(key, maxRequests, windowSeconds) {
    const fylo = await this.fyloHelper.database()
    const hash = RateLimitRules.hashKey(key)
    const now = Math.floor(Date.now() / 1000)
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.RATE_LIMITS, { $ops: [{ key: { $eq: hash } }] }).collect())
    const entry = Object.entries(docs)[0]
    let count = 1
    let resetAt = now + windowSeconds

    if (entry) {
      const [docId, record] = entry
      const r = /** @type {{ key: string, count: number, resetAt: number }} */ (record)
      if (r.resetAt <= now) {
        await fylo.patchDoc(Fylo.Collections.RATE_LIMITS, { [docId]: { count: 1, resetAt } })
      } else {
        count = r.count + 1
        resetAt = r.resetAt
        await fylo.patchDoc(Fylo.Collections.RATE_LIMITS, { [docId]: { count } })
      }
    } else {
      await fylo.putData(Fylo.Collections.RATE_LIMITS, { key: hash, count: 1, resetAt })
    }

    return RateLimitRules.formatResult(count, maxRequests, resetAt, now)
  }
}

