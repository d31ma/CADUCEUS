import { RouteRequest } from '../shared/http/RouteRequest.js'
import { createMcpHandler } from '../deps/npm/modelcontextprotocol-server.mjs'
import { Tools } from '../shared/mcp/Tools.js'

@RequestRoute("/mcp")
@Public
export class McpController {
  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /** @type {DraftService} */
  @Needs
  draftService

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {McpAccessService} */
  @Needs
  mcpAccessService

  /** @type {McpAuditEventsHelper} */
  @Needs
  mcpAuditEventsHelper

  /** @type {RateLimitService} */
  @Needs
  rateLimitService

  /** @type {ScheduledHelper} */
  @Needs
  scheduledHelper

  /** @type {SendService} */
  @Needs
  sendService

  /** @type {SuppressedHelper} */
  @Needs
  suppressedHelper

  /** @type {ThreadsHelper} */
  @Needs
  threadsHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  static MAX_REQUEST_BYTES = 1024 * 1024

  static AUTH_LIMIT_PER_MINUTE = 600

  static toHeaders(source) {
    const headers = new Headers()
    if (!source || typeof source !== 'object') return headers
    for (const [key, raw] of Object.entries(source)) {
      if (!/^[!#$%&\x27*+.^_\x60|~0-9A-Za-z-]+$/.test(key)) continue
      const value = Array.isArray(raw) ? raw.join(', ') : String(raw ?? '')
      if (/[\r\n]/.test(value)) continue
      headers.set(key, value)
    }
    return headers
  }

  static validateOrigin(headers) {
    const origin = headers.get('origin')
    if (!origin) return null
    const host = headers.get('host')
    try {
      if (!host || new URL(origin).host.toLowerCase() !== host.toLowerCase()) {
        return McpController.jsonResponse(403, { error: 'Cross-origin MCP requests are not allowed' })
      }
    } catch { return McpController.jsonResponse(403, { error: 'Cross-origin MCP requests are not allowed' }) }
    return null
  }

  static bearerToken(value) {
    const match = String(value || '').match(/^Bearer ([^\s]{1,420})$/i)
    return match?.[1] || ''
  }

  static isJson(value) { return /^application\/json(?:\s*;|$)/i.test(String(value || '')) }

  static jsonResponse(status, body) {
    return new Response(JSON.stringify(body), {
      status,
      headers: { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' },
    })
  }

  /**
   * Stateless MCP endpoint. Authentication happens before the protocol SDK is
   * invoked, using a separately issued hash-only MCP capability rather than a
   * browser session. The SDK serves the current 2026 protocol and its
   * stateless 2025-era compatibility path from the same tool factory.
   */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, {  }, async ({ body, headers = {}, context = {} }) => {
      const app = this
      const normalizedHeaders = McpController.toHeaders(headers)
      const originFailure = McpController.validateOrigin(normalizedHeaders)
      if (originFailure) return originFailure
      if (!McpController.isJson(normalizedHeaders.get('content-type'))) {
        return McpController.jsonResponse(415, { error: 'Content-Type must be application/json' })
      }

      const serialized = JSON.stringify(body ?? null)
      if (new TextEncoder().encode(serialized).byteLength > McpController.MAX_REQUEST_BYTES) {
        return McpController.jsonResponse(413, { error: 'MCP request is too large' })
      }
      const limit = await app.rateLimitService.check(
        ['mcp-auth', String(context.ipAddress || 'unknown')], McpController.AUTH_LIMIT_PER_MINUTE, 60_000,
      )
      if (!limit.allowed) return McpController.jsonResponse(429, { error: 'Too many MCP requests', retryAfterSeconds: limit.retryAfterSeconds })

      const token = McpController.bearerToken(normalizedHeaders.get('authorization'))
      const principal = token ? await app.mcpAccessService.authenticate(token) : null
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
      const capabilityLimit = await app.rateLimitService.check(
        ['mcp-request', principal.tokenId], McpController.AUTH_LIMIT_PER_MINUTE, 60_000,
      )
      if (!capabilityLimit.allowed) {
        return McpController.jsonResponse(429, {
          error: 'Too many MCP requests',
          retryAfterSeconds: capabilityLimit.retryAfterSeconds,
        })
      }

      // The MCP transport answers in process; the Request never leaves this server.
      const mcpRequest = new Request(new URL('/mcp', request.url), {
        method: 'POST', headers: normalizedHeaders, body: serialized,
      })
      const handler = createMcpHandler(
        () => Tools.createMailboxMcpServer(app, principal),
        { legacy: 'stateless', responseMode: 'json', keepAliveMs: 0 },
      )
      try {
        const response = await handler.fetch(mcpRequest, { parsedBody: body })
        const safeHeaders = new Headers(response.headers)
        safeHeaders.set('cache-control', 'no-store')
        safeHeaders.set('x-content-type-options', 'nosniff')
        return new Response(response.body, { status: response.status, statusText: response.statusText, headers: safeHeaders })
      } finally {
        await handler.close()
      }
    })
  }
}
