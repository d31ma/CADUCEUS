import { createHash, randomBytes, timingSafeEqual } from 'node:crypto'
import { Buffer } from 'node:buffer'
import { FyloAccessContext } from '../../../platform/persistence/fylo-access.js'
import { Security } from '../../../platform/security/security.js'
import { normalizeMcpPreferences, validateMcpPreferences } from './preferences.js'

const TOKEN_BYTES = 32
const MAX_OLD_TOKENS = 5

export class McpAccessService {
  /**
   * @param {import('../repositories/access-tokens.js').McpAccessTokensRepository} tokens
   * @param {import('../repositories/audit-events.js').McpAuditEventsRepository} auditEvents
   * @param {import('../../users/repositories/users.js').UsersRepository} users
   */
  constructor(tokens, auditEvents, users) {
    this.tokens = tokens
    this.auditEvents = auditEvents
    this.users = users
  }

  /** @param {string} ownerEmail */
  async config(ownerEmail) {
    const [, user] = await this.users.findByEmail(ownerEmail)
    if (!user) return null
    const preferences = normalizeMcpPreferences(user.mcp, user.domains)
    const records = await this.tokens.listForOwner(ownerEmail)
    const active = records.find(record => record.active === true && !record.revokedAt)
    return {
      ...preferences,
      hasToken: Boolean(active),
      tokenCreatedAt: active?.createdAt || '',
      endpoint: '/mcp',
      protocolVersions: ['2026-07-28', '2025-11-25', '2025-06-18', '2025-03-26', '2024-11-05', '2024-10-07'],
    }
  }

  /** @param {string} ownerEmail @param {unknown} patch */
  async updateConfig(ownerEmail, patch) {
    const [docId, user] = await this.users.findByEmail(ownerEmail)
    if (!docId || !user) return null
    const current = normalizeMcpPreferences(user.mcp, user.domains)
    const merged = patch && typeof patch === 'object' && !Array.isArray(patch)
      ? { ...current, ...patch }
      : patch
    const parsed = validateMcpPreferences(merged, user.domains)
    if (!parsed.ok) throw new McpPreferenceError(parsed.error, parsed.forbidden)
    await this.users.updateMcpPreferences(docId, parsed.preferences)
    return await this.config(ownerEmail)
  }

  /** Rotate the owner capability, returning plaintext exactly once. */
  async rotate(ownerEmail) {
    const [userDocId, user] = await this.users.findByEmail(ownerEmail)
    if (!userDocId || !user) return null
    const domain = domainOf(ownerEmail)
    if (!domain) return null
    const now = new Date().toISOString()
    const existing = (await this.tokens.listForOwner(ownerEmail))
      .sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))
    for (const record of existing.filter(record => record.active === true && !record.revokedAt)) {
      await this.tokens.update(record.docId, { active: false, revokedAt: now })
    }
    for (const stale of existing.slice(MAX_OLD_TOKENS - 1)) await this.tokens.delete(stale.docId)

    const token = createToken(domain)
    const record = {
      id: `mcp-token-${randomBytes(16).toString('base64url')}`,
      ownerEmail: String(ownerEmail).toLowerCase(),
      domain,
      tokenHash: hashToken(token),
      active: true,
      createdAt: now,
    }
    await this.tokens.put(record)
    return { token, tokenId: record.id, createdAt: record.createdAt }
  }

  /** @param {string} ownerEmail */
  async revoke(ownerEmail) {
    const now = new Date().toISOString()
    const records = await this.tokens.listForOwner(ownerEmail)
    let revoked = 0
    for (const record of records.filter(item => item.active === true && !item.revokedAt)) {
      await this.tokens.update(record.docId, { active: false, revokedAt: now })
      revoked++
    }
    return revoked > 0
  }

  /** Resolve an MCP bearer without trusting a caller-selected tenant. */
  async authenticate(token) {
    const parsed = parseToken(token)
    if (!parsed) return null
    return await FyloAccessContext.runForDomain(parsed.domain, async () => {
      const candidates = await this.tokens.findByHash(parsed.hash)
      const record = candidates.find(candidate =>
        candidate.active === true && !candidate.revokedAt && candidate.domain === parsed.domain
        && equalHash(String(candidate.tokenHash || ''), parsed.hash))
      if (!record) return null
      const [, user] = await this.users.findByEmail(record.ownerEmail)
      if (!user) return null
      const preferences = normalizeMcpPreferences(user.mcp, user.domains)
      if (!preferences.enabled || preferences.allowedDomains.length === 0) return null
      await this.tokens.update(record.docId, { lastUsedAt: new Date().toISOString() })
      return {
        ownerEmail: record.ownerEmail,
        tokenId: record.id,
        primaryDomain: parsed.domain,
        ...preferences,
      }
    })
  }
}

export class McpPreferenceError extends Error {
  constructor(message, forbidden = false) { super(message); this.forbidden = forbidden }
}

function createToken(domain) {
  return `mcp1.${Buffer.from(domain, 'utf8').toString('base64url')}.${randomBytes(TOKEN_BYTES).toString('base64url')}`
}

function parseToken(value) {
  const token = typeof value === 'string' ? value.trim() : ''
  if (token.length > 420) return null
  const match = token.match(/^mcp1\.([A-Za-z0-9_-]{4,340})\.([A-Za-z0-9_-]{43})$/)
  if (!match) return null
  let decoded = ''
  try { decoded = Buffer.from(match[1], 'base64url').toString('utf8') } catch { return null }
  const domain = Security.normalizeDomain(decoded)
  if (!domain || Buffer.from(domain, 'utf8').toString('base64url') !== match[1]) return null
  return { domain, hash: hashToken(token) }
}

function hashToken(token) { return createHash('sha256').update(token).digest('hex') }

function equalHash(left, right) {
  if (!/^[a-f0-9]{64}$/.test(left) || !/^[a-f0-9]{64}$/.test(right)) return false
  return timingSafeEqual(Buffer.from(left, 'hex'), Buffer.from(right, 'hex'))
}

function domainOf(email) {
  const normalized = Security.normalizeEmailAddress(email)
  return normalized?.split('@')[1] || ''
}
