import { Fylo } from '../persistence/Fylo.js'
import { Security } from '../security/Security.js'

export class RateLimitRules {
  /** @param {string | string[]} key @returns {string} */
  static hashKey(key) {
    return Security.sha256Hex(Array.isArray(key) ? key.join(':') : String(key))
  }

  /**
   * @param {number} count
   * @param {number} maxRequests
   * @param {number} resetAt
   * @param {number} now
   * @returns {RateLimitResult}
   */
  static formatResult(count, maxRequests, resetAt, now) {
    return {
      allowed: count <= maxRequests,
      limit: maxRequests,
      remaining: Math.max(0, maxRequests - count),
      resetAt,
      retryAfterSeconds: resetAt - now,
    }
  }
}
