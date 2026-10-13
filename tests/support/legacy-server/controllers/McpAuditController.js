import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'

@RequestRoute("/mcp/audit")
@SignedIn
export class McpAuditController {
  /** @type {McpAuditEventsHelper} */
  @Needs
  mcpAuditEventsHelper

  static present(record) {
    return {
      id: record.id,
      tokenId: record.tokenId,
      toolName: record.toolName,
      category: record.category,
      status: record.status,
      parameterKeys: record.parameterKeys,
      resultCount: record.resultCount,
      durationMs: record.durationMs,
      createdAt: record.createdAt,
    }
  }

  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_READ } }, async ({ auth }) => {
      const app = this
      return (await app.mcpAuditEventsHelper.listForOwner(auth.email)).map(McpAuditController.present)
    })
  }

  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ auth }) => {
      const app = this
      return { cleared: await app.mcpAuditEventsHelper.clearForOwner(auth.email) }
    })
  }
}
