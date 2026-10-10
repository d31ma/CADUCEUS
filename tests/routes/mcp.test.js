import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { randomBytes } from 'node:crypto'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { McpConfigController as Config } from '@/controllers/McpConfigController.js'
import { McpTokenController as Token } from '@/controllers/McpTokenController.js'
import { McpAuditController as Audit } from '@/controllers/McpAuditController.js'
import { McpController as Mcp } from '@/controllers/McpController.js'
import { FyloAccessContext } from '@/platform/persistence/FyloAccessContext.js'
import { call } from '../support/controller-call.mjs'

let app
let firstToken = ''
let secondToken = ''

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({
    email: 'alice@example.com',
    aliases: [],
    domains: ['example.com'],
    role: 'admin',
  })
}, 300_000)
afterAll(teardownRoutes)

describe('mailbox-scoped MCP access boundary', () => {
  it('starts disabled, requires a SESAME session, and exposes no secret', async () => {
    expect((await call(Config, 'GET', req())).unauthorized).toBe(true)
    expect(await call(Config, 'GET', req({ token: token() }))).toEqual({
      enabled: false,
      accessLevel: 'read-only',
      allowedDomains: ['example.com'],
      excludedFolders: {},
      hasToken: false,
      tokenCreatedAt: '',
      endpoint: '/mcp',
      protocolVersions: ['2026-07-28', '2025-11-25', '2025-06-18', '2025-03-26', '2024-11-05', '2024-10-07'],
    })
  })

  it('rejects widened domains, unsafe exclusions, unknown fields, and empty enabled scope', async () => {
    expect((await call(Config, 'PUT', req({ token: token(), body: { accessLevel: 'root' } }))).error).toContain('accessLevel')
    expect((await call(Config, 'PUT', req({
      token: token(),
      body: { allowedDomains: ['other.com'] },
    }))).forbidden).toBe(true)
    expect((await call(Config, 'PUT', req({
      token: token(),
      body: { enabled: true, allowedDomains: [] },
    }))).error).toContain('At least one')
    expect((await call(Config, 'PUT', req({
      token: token(),
      body: { excludedFolders: { 'example.com': ['../secret'] } },
    }))).error).toContain('safe bounded')
    expect((await call(Config, 'PUT', req({ token: token(), body: { mystery: true } }))).error).toContain('unknown field')
  })

  it('persists explicit account, access-level, and folder scope', async () => {
    const result = await call(Config, 'PUT', req({ token: token(), body: {
      enabled: true,
      accessLevel: 'read-write',
      allowedDomains: ['EXAMPLE.COM'],
      excludedFolders: { 'example.com': ['spam', 'private/projects'] },
    } }))
    expect(result).toMatchObject({
      enabled: true,
      accessLevel: 'read-write',
      allowedDomains: ['example.com'],
      excludedFolders: { 'example.com': ['spam', 'private/projects'] },
      hasToken: false,
    })
  })

  it('returns a capability once, stores only its hash, and rotates immediately', async () => {
    const first = await call(Token, 'POST', req({ token: token() }))
    firstToken = first.token
    expect(firstToken).toMatch(/^mcp1\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]{43}$/)
    const records = await FyloAccessContext.runForDomain('example.com', () => app.mcpAccessTokens.listForOwner('alice@example.com'))
    expect(records).toHaveLength(1)
    expect(records[0].tokenHash).toMatch(/^[a-f0-9]{64}$/)
    expect(JSON.stringify(records[0])).not.toContain(firstToken)

    const authenticated = await app.mcpAccess.authenticate(firstToken)
    expect(authenticated).toMatchObject({
      ownerEmail: 'alice@example.com',
      accessLevel: 'read-write',
      allowedDomains: ['example.com'],
    })
    const altered = `${firstToken.slice(0, -1)}${firstToken.endsWith('A') ? 'B' : 'A'}`
    expect(await app.mcpAccess.authenticate(altered)).toBeNull()

    const second = await call(Token, 'POST', req({ token: token() }))
    secondToken = second.token
    expect(secondToken).not.toBe(firstToken)
    expect(await app.mcpAccess.authenticate(firstToken)).toBeNull()
    expect(await app.mcpAccess.authenticate(secondToken)).not.toBeNull()
    expect((await call(Config, 'GET', req({ token: token() }))).hasToken).toBe(true)
  })

  it('lists only redacted audit metadata and clears the exact owner log', async () => {
    await FyloAccessContext.runForDomain('example.com', () => app.mcpAuditEvents.put({
      id: `mcp-audit-${randomBytes(16).toString('base64url')}`,
      ownerEmail: 'alice@example.com',
      domain: 'example.com',
      tokenId: `mcp-token-${randomBytes(16).toString('base64url')}`,
      toolName: 'search_mail',
      category: 'read',
      status: 'ok',
      parameterKeysJson: JSON.stringify(['limit', 'query']),
      resultCount: 2,
      durationMs: 4,
      createdAt: new Date().toISOString(),
    }))
    const entries = await call(Audit, 'GET', req({ token: token() }))
    expect(entries).toHaveLength(1)
    expect(entries[0]).toMatchObject({ toolName: 'search_mail', parameterKeys: ['limit', 'query'], resultCount: 2 })
    expect(entries[0]).not.toHaveProperty('ownerEmail')
    expect(entries[0]).not.toHaveProperty('parameterKeysJson')
    expect((await call(Audit, 'DELETE', req({ token: token() }))).cleared).toBe(1)
    expect(await call(Audit, 'GET', req({ token: token() }))).toEqual([])
  })

  it('serves the official stateless protocol and enforces tool access levels', async () => {
    const headers = {
      authorization: `Bearer ${secondToken}`,
      'content-type': 'application/json',
      accept: 'application/json, text/event-stream',
    }
    const listResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 1, method: 'tools/list', params: {},
    } }))
    expect(listResponse).toBeInstanceOf(Response)
    expect(listResponse.status).toBe(200)
    const listed = await mcpJson(listResponse)
    const names = listed.result.tools.map(tool => tool.name)
    expect(names).toContain('list_accounts')
    expect(names).toContain('schedule_send')
    expect(names).toHaveLength(21)

    const callResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 2, method: 'tools/call',
      params: { name: 'list_accounts', arguments: {} },
    } }))
    const called = await mcpJson(callResponse)
    expect(called.result.isError).not.toBe(true)
    expect(called.result.structuredContent.items[0]).toMatchObject({
      id: 'example.com', provider: 'caduceus', usesLabels: true,
    })

    const message = email({ id: 'mcp-label-message', threadId: 'mcp-label-thread', labels: ['old'] })
    await app.emails.put(message)
    const labelResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 21, method: 'tools/call',
      params: {
        name: 'set_labels',
        arguments: { threadIds: ['mcp-label-thread'], labels: ['projects/alpha', 'important'] },
      },
    } }))
    const labeled = await mcpJson(labelResponse)
    expect(labeled.result.isError).not.toBe(true)
    expect(labeled.result.structuredContent).toEqual({ updatedThreads: 1, updatedMessages: 1 })
    expect((await app.emails.findById(message.id))[1]?.labels).toEqual(['projects/alpha', 'important'])

    const invalidLabelResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 22, method: 'tools/call',
      params: {
        name: 'set_labels',
        arguments: { threadIds: ['mcp-label-thread'], labels: ['../private'] },
      },
    } }))
    const invalidLabel = await mcpJson(invalidLabelResponse)
    expect(invalidLabel.result.isError).toBe(true)
    expect(invalidLabel.result.content[0].text).toContain('safe label path')
    expect((await app.emails.findById(message.id))[1]?.labels).toEqual(['projects/alpha', 'important'])

    const duplicateLabelResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 23, method: 'tools/call',
      params: {
        name: 'set_labels',
        arguments: { threadIds: ['mcp-label-thread'], labels: ['important', 'important'] },
      },
    } }))
    const duplicateLabel = await mcpJson(duplicateLabelResponse)
    expect(duplicateLabel.result.isError).toBe(true)
    expect(duplicateLabel.result.content[0].text).toContain('labels must be unique')

    const deniedResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 3, method: 'tools/call',
      params: { name: 'send_draft', arguments: { draftId: 'missing' } },
    } }))
    const denied = await mcpJson(deniedResponse)
    expect(denied.result.isError).toBe(true)
    expect(denied.result.content[0].text).toContain('does not permit send')

    const unsafeFolderResponse = await call(Mcp, 'POST', req({ headers, body: {
      jsonrpc: '2.0', id: 4, method: 'tools/call',
      params: {
        name: 'move_to_folder',
        arguments: { threadIds: ['thread-1'], accountId: 'example.com', folder: '../private' },
      },
    } }))
    const unsafeFolder = await mcpJson(unsafeFolderResponse)
    expect(unsafeFolder.result.isError).toBe(true)
    expect(unsafeFolder.result.content[0].text).toContain('safe folder path')

    const missingType = await call(Mcp, 'POST', req({ headers: { authorization: `Bearer ${secondToken}` }, body: {} }))
    expect(missingType.status).toBe(415)
    const invalid = await call(Mcp, 'POST', req({ headers: { ...headers, authorization: 'Bearer invalid' }, body: {
      jsonrpc: '2.0', id: 5, method: 'tools/list', params: {},
    } }))
    expect(invalid.status).toBe(401)
  })

  it('revokes the active capability and disabling fails closed', async () => {
    expect((await call(Token, 'DELETE', req({ token: token() }))).revoked).toBe(true)
    expect(await app.mcpAccess.authenticate(secondToken)).toBeNull()
    const third = await call(Token, 'POST', req({ token: token() }))
    await call(Config, 'PUT', req({ token: token(), body: { enabled: false } }))
    const config = await call(Config, 'GET', req({ token: token() }))
    expect(config).toMatchObject({ enabled: false, hasToken: true })
    const records = await FyloAccessContext.runForDomain('example.com', () => app.mcpAccessTokens.listForOwner('alice@example.com'))
    const active = records.find(record => record.active && !record.revokedAt)
    expect(active).toBeTruthy()
    expect(await app.mcpAccess.authenticate(third.token)).toBeNull()
  })
})

async function mcpJson(response) {
  const text = await response.text()
  if (!String(response.headers.get('content-type')).includes('text/event-stream')) return JSON.parse(text)
  const payloads = text.split(/\r?\n/)
    .filter(line => line.startsWith('data: '))
    .map(line => JSON.parse(line.slice(6)))
  return payloads.at(-1)
}
