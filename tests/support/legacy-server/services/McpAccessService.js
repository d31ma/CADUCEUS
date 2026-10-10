import { FyloAccessContext } from '../shared/persistence/FyloAccessContext.js'
import { Security } from '../shared/security/Security.js'
import { Preferences } from '../shared/mcp/Preferences.js'
import { McpAccessRules } from '../shared/mcp/McpAccessRules.js'
import { McpPreferenceError } from '../shared/errors/McpPreferenceError.js'

@Service
export default class McpAccessService {
  /** @type {McpAccessTokensHelper} */
  @Needs
  mcpAccessTokensHelper

  /** @type {McpAuditEventsHelper} */
  @Needs
  mcpAuditEventsHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /** @param {string} ownerEmail */
  async config(ownerEmail) {
    const [, user] = await this.usersHelper.findByEmail(ownerEmail)
    if (!user) return null
    const preferences = Preferences.normalizeMcpPreferences(user.mcp, user.domains)
    const records = await this.mcpAccessTokensHelper.listForOwner(ownerEmail)
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
    const [docId, user] = await this.usersHelper.findByEmail(ownerEmail)
    if (!docId || !user) return null
    const current = Preferences.normalizeMcpPreferences(user.mcp, user.domains)
    const merged = patch && typeof patch === 'object' && !Array.isArray(patch)
      ? { ...current, ...patch }
      : patch
    const parsed = Preferences.validateMcpPreferences(merged, user.domains)
    if (!parsed.ok) throw new McpPreferenceError(parsed.error, parsed.forbidden)
    await this.usersHelper.updateMcpPreferences(docId, parsed.preferences)
    return await this.config(ownerEmail)
  }

  /** Rotate the owner capability, returning plaintext exactly once. */
  async rotate(ownerEmail) {
    const [userDocId, user] = await this.usersHelper.findByEmail(ownerEmail)
    if (!userDocId || !user) return null
    const domain = McpAccessRules.domainOf(ownerEmail)
    if (!domain) return null
    const now = new Date().toISOString()
    const existing = (await this.mcpAccessTokensHelper.listForOwner(ownerEmail))
      .sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))
    for (const record of existing.filter(record => record.active === true && !record.revokedAt)) {
      await this.mcpAccessTokensHelper.update(record.docId, { active: false, revokedAt: now })
    }
    for (const stale of existing.slice(McpAccessRules.MAX_OLD_TOKENS - 1)) await this.mcpAccessTokensHelper.delete(stale.docId)

    const token = McpAccessRules.createToken(domain)
    const record = {
      id: `mcp-token-${Security.randomText(16, 'base64url')}`,
      ownerEmail: String(ownerEmail).toLowerCase(),
      domain,
      tokenHash: McpAccessRules.hashToken(token),
      active: true,
      createdAt: now,
    }
    await this.mcpAccessTokensHelper.put(record)
    return { token, tokenId: record.id, createdAt: record.createdAt }
  }

  /** @param {string} ownerEmail */
  async revoke(ownerEmail) {
    const now = new Date().toISOString()
    const records = await this.mcpAccessTokensHelper.listForOwner(ownerEmail)
    let revoked = 0
    for (const record of records.filter(item => item.active === true && !item.revokedAt)) {
      await this.mcpAccessTokensHelper.update(record.docId, { active: false, revokedAt: now })
      revoked++
    }
    return revoked > 0
  }

  /** Resolve an MCP bearer without trusting a caller-selected tenant. */
  async authenticate(token) {
    const parsed = McpAccessRules.parseToken(token)
    if (!parsed) return null
    return await FyloAccessContext.runForDomain(parsed.domain, async () => {
      const candidates = await this.mcpAccessTokensHelper.findByHash(parsed.hash)
      const record = candidates.find(candidate =>
        candidate.active === true && !candidate.revokedAt && candidate.domain === parsed.domain
        && McpAccessRules.equalHash(String(candidate.tokenHash || ''), parsed.hash))
      if (!record) return null
      const [, user] = await this.usersHelper.findByEmail(record.ownerEmail)
      if (!user) return null
      const preferences = Preferences.normalizeMcpPreferences(user.mcp, user.domains)
      if (!preferences.enabled || preferences.allowedDomains.length === 0) return null
      await this.mcpAccessTokensHelper.update(record.docId, { lastUsedAt: new Date().toISOString() })
      return {
        ownerEmail: record.ownerEmail,
        tokenId: record.id,
        primaryDomain: parsed.domain,
        ...preferences,
      }
    })
  }
}

