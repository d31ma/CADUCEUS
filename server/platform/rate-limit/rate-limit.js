import { Collections, collect } from '../persistence/fylo.js'
import { Security } from '../security/security.js'

/** @typedef {{ allowed: boolean, limit: number, remaining: number, resetAt: number, retryAfterSeconds: number }} RateLimitResult */

/**
 * Fylo-backed rate limiter shared by the self-hosted application processes.
 */
export class RateLimitService {
  /** @param {import('../persistence/fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

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
    const hash = hashKey(key)
    const now = Math.floor(Date.now() / 1000)
    const docs = await collect(this.fylo.findDocs(Collections.RATE_LIMITS, { $ops: [{ key: { $eq: hash } }] }).collect())
    const entry = Object.entries(docs)[0]
    let count = 1
    let resetAt = now + windowSeconds

    if (entry) {
      const [docId, record] = entry
      const r = /** @type {{ key: string, count: number, resetAt: number }} */ (record)
      if (r.resetAt <= now) {
        await this.fylo.patchDoc(Collections.RATE_LIMITS, { [docId]: { count: 1, resetAt } })
      } else {
        count = r.count + 1
        resetAt = r.resetAt
        await this.fylo.patchDoc(Collections.RATE_LIMITS, { [docId]: { count } })
      }
    } else {
      await this.fylo.putData(Collections.RATE_LIMITS, { key: hash, count: 1, resetAt })
    }

    return formatResult(count, maxRequests, resetAt, now)
  }
}

/** @param {string | string[]} key @returns {string} */
function hashKey(key) {
  return Security.sha256Hex(Array.isArray(key) ? key.join(':') : String(key))
}

/**
 * @param {number} count
 * @param {number} maxRequests
 * @param {number} resetAt
 * @param {number} now
 * @returns {RateLimitResult}
 */
function formatResult(count, maxRequests, resetAt, now) {
  return {
    allowed: count <= maxRequests,
    limit: maxRequests,
    remaining: Math.max(0, maxRequests - count),
    resetAt,
    retryAfterSeconds: resetAt - now,
  }
}
