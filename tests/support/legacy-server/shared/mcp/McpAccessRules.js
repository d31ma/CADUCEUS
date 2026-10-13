import { createHash, randomBytes, timingSafeEqual } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { FyloAccessContext } from '../persistence/FyloAccessContext.js'
import { Security } from '../security/Security.js'
import { Preferences } from './Preferences.js'

export class McpAccessRules {
  static TOKEN_BYTES = 32

  static MAX_OLD_TOKENS = 5

  static createToken(domain) {
    return `mcp1.${Buffer.from(domain, 'utf8').toString('base64url')}.${randomBytes(McpAccessRules.TOKEN_BYTES).toString('base64url')}`
  }

  static parseToken(value) {
    const token = typeof value === 'string' ? value.trim() : ''
    if (token.length > 420) return null
    const match = token.match(/^mcp1\.([A-Za-z0-9_-]{4,340})\.([A-Za-z0-9_-]{43})$/)
    if (!match) return null
    let decoded = ''
    try { decoded = Buffer.from(match[1], 'base64url').toString('utf8') } catch { return null }
    const domain = Security.normalizeDomain(decoded)
    if (!domain || Buffer.from(domain, 'utf8').toString('base64url') !== match[1]) return null
    return { domain, hash: McpAccessRules.hashToken(token) }
  }

  static hashToken(token) { return createHash('sha256').update(token).digest('hex') }

  static equalHash(left, right) {
    if (!/^[a-f0-9]{64}$/.test(left) || !/^[a-f0-9]{64}$/.test(right)) return false
    return timingSafeEqual(Buffer.from(left, 'hex'), Buffer.from(right, 'hex'))
  }

  static domainOf(email) {
    const normalized = Security.normalizeEmailAddress(email)
    return normalized?.split('@')[1] || ''
  }
}
