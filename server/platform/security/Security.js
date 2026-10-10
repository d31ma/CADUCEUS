import { createHash, createHmac, timingSafeEqual } from 'node:crypto'
import { isIP } from 'node:net'

const CONTROL_CHARS = /[\0\r\n]/

/** Stateless security/validation helpers (hashing, normalization, SSRF guards). */
export class Security {
  /** @param {string} value @returns {boolean} */
  static hasControlChars(value) {
    return CONTROL_CHARS.test(value)
  }

  /** @param {string} s @returns {string} */
  static sha256Hex(s) {
    return createHash('sha256').update(s).digest('hex')
  }

  /**
   * Constant-time string comparison. Returns `false` on length mismatch or
   * invalid input rather than throwing.
   * @param {string} a
   * @param {string} b
   * @returns {boolean}
   */
  static timingSafeStringEqual(a, b) {
    try {
      return timingSafeEqual(Buffer.from(a), Buffer.from(b))
    } catch {
      return false
    }
  }

  /** @param {string} data @param {string} secret @returns {string} */
  static hmacSha256Hex(data, secret) {
    return createHmac('sha256', secret).update(data).digest('hex')
  }

  /** @param {string} name @returns {string} */
  static requireEnv(name) {
    const v = process.env[name]
    if (!v) throw new Error(`${name} is required`)
    return v
  }

  /** @param {string} email @returns {string | null} */
  static normalizeEmailAddress(email) {
    const t = String(email ?? '').trim().toLowerCase()
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(t) ? t : null
  }

  /** @param {string} domain @returns {string | null} */
  static normalizeDomain(domain) {
    const t = String(domain ?? '').trim().toLowerCase().replace(/\.+$/, '')
    return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*\.[a-z]{2,}$/.test(t) ? t : null
  }

  /**
   * Block webhook URLs that would let a misconfigured rule point at the server's
   * own loopback or RFC-1918 private network. Returns the parsed host on
   * success so callers can log it.
   *
   * @param {string} url
   * @returns {{ ok: true, host: string } | { ok: false, error: string }}
   */
  static assertSafeWebhookUrl(url) {
    try {
      const u = new URL(url)
      if (u.protocol !== 'http:' && u.protocol !== 'https:') return { ok: false, error: 'Unsafe protocol' }
      const host = u.hostname.startsWith('[') ? u.hostname.slice(1, -1) : u.hostname
      if (host === 'localhost' || host === '127.0.0.1' || host === '::1' || host === '0.0.0.0') {
        return { ok: false, error: 'Webhook URL must not be localhost' }
      }
      if (isIP(host) && Security.#isPrivateIPv4(host)) {
        return { ok: false, error: 'Webhook URL must not be a private address' }
      }
      return { ok: true, host }
    } catch (e) {
      return { ok: false, error: /** @type {Error} */ (e).message || 'Invalid URL' }
    }
  }

  /**
   * Whether the test-only routes (`/test/seed/*`, `/test/reset`) are active.
   *
   * Gated solely on the explicit `CADUCEUS_ENABLE_TEST_ROUTES` opt-in flag — the
   * flag name is self-documenting and an operator must deliberately set it.
   * `NODE_ENV` is intentionally NOT part of the check: the API test harness runs
   * the server under `NODE_ENV=production` (to exercise production config) while
   * still needing the seed endpoints. Production *binaries* never expose these
   * routes regardless — `scripts/compile.mjs` strips them from the bundle since
   * the flag is unset at build time.
   *
   * @returns {boolean}
   */
  static isTestRoutesEnabled() {
    return process.env.CADUCEUS_ENABLE_TEST_ROUTES === 'true'
  }

  /** @param {string[]} userDomains @param {string} domain @returns {boolean} */
  static hasDomainClaim(userDomains, domain) {
    if (!domain || !Array.isArray(userDomains)) return false
    return userDomains.some(d => d.toLowerCase() === domain.toLowerCase())
  }

  /** @param {string} ip @returns {boolean} */
  static #isPrivateIPv4(ip) {
    if (ip.startsWith('10.') || ip.startsWith('192.168.')) return true
    if (ip.startsWith('172.')) {
      const second = parseInt(ip.split('.')[1], 10)
      return second >= 16 && second <= 31
    }
    return false
  }
}
