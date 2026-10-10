import { createMcpHandler } from '@modelcontextprotocol/server'
import { getApp } from '../../app.js'
import { createMailboxMcpServer } from '../../features/mcp/services/tools.js'

const MAX_REQUEST_BYTES = 1024 * 1024
const AUTH_LIMIT_PER_MINUTE = 600

export class Handler {
  /**
   * Stateless MCP endpoint. Authentication happens before the protocol SDK is
   * invoked, using a separately issued hash-only MCP capability rather than a
   * browser session. The SDK serves the current 2026 protocol and its
   * stateless 2025-era compatibility path from the same tool factory.
   */
  static async POST(/** @type {any} */ { body, headers = {}, context = {} }) {
    const app = await getApp()
    const normalizedHeaders = toHeaders(headers)
    const originFailure = validateOrigin(normalizedHeaders)
    if (originFailure) return originFailure
    if (!isJson(normalizedHeaders.get('content-type'))) {
      return jsonResponse(415, { error: 'Content-Type must be application/json' })
    }

    const serialized = JSON.stringify(body ?? null)
    if (new TextEncoder().encode(serialized).byteLength > MAX_REQUEST_BYTES) {
      return jsonResponse(413, { error: 'MCP request is too large' })
    }
    const limit = await app.rateLimit.check(
      ['mcp-auth', String(context.ipAddress || 'unknown')], AUTH_LIMIT_PER_MINUTE, 60_000,
    )
    if (!limit.allowed) return jsonResponse(429, { error: 'Too many MCP requests', retryAfterSeconds: limit.retryAfterSeconds })

    const token = bearerToken(normalizedHeaders.get('authorization'))
    const principal = token ? await app.mcpAccess.authenticate(token) : null
    if (!principal) {
      return new Response(JSON.stringify({ error: 'Invalid or disabled MCP capability' }), {
        status: 401,
        headers: {
          'content-type': 'application/json; charset=utf-8',
          'www-authenticate': 'Bearer realm="caduceus-mcp"',
          'cache-control': 'no-store',
        },
      })
    }
    const capabilityLimit = await app.rateLimit.check(
      ['mcp-request', principal.tokenId], AUTH_LIMIT_PER_MINUTE, 60_000,
    )
    if (!capabilityLimit.allowed) {
      return jsonResponse(429, {
        error: 'Too many MCP requests',
        retryAfterSeconds: capabilityLimit.retryAfterSeconds,
      })
    }

    const request = new Request('http://caduceus.invalid/mcp', {
      method: 'POST', headers: normalizedHeaders, body: serialized,
    })
    const handler = createMcpHandler(
      () => createMailboxMcpServer(app, principal),
      { legacy: 'stateless', responseMode: 'json', keepAliveMs: 0 },
    )
    try {
      const response = await handler.fetch(request, { parsedBody: body })
      const safeHeaders = new Headers(response.headers)
      safeHeaders.set('cache-control', 'no-store')
      safeHeaders.set('x-content-type-options', 'nosniff')
      return new Response(response.body, { status: response.status, statusText: response.statusText, headers: safeHeaders })
    } finally {
      await handler.close()
    }
  }
}

function toHeaders(source) {
  const headers = new Headers()
  if (!source || typeof source !== 'object') return headers
  for (const [key, raw] of Object.entries(source)) {
    if (!/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(key)) continue
    const value = Array.isArray(raw) ? raw.join(', ') : String(raw ?? '')
    if (/[\r\n]/.test(value)) continue
    headers.set(key, value)
  }
  return headers
}

function validateOrigin(headers) {
  const origin = headers.get('origin')
  if (!origin) return null
  const host = headers.get('host')
  try {
    if (!host || new URL(origin).host.toLowerCase() !== host.toLowerCase()) {
      return jsonResponse(403, { error: 'Cross-origin MCP requests are not allowed' })
    }
  } catch { return jsonResponse(403, { error: 'Cross-origin MCP requests are not allowed' }) }
  return null
}

function bearerToken(value) {
  const match = String(value || '').match(/^Bearer ([^\s]{1,420})$/i)
  return match?.[1] || ''
}

function isJson(value) { return /^application\/json(?:\s*;|$)/i.test(String(value || '')) }

function jsonResponse(status, body) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' },
  })
}
