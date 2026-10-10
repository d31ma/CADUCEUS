import { RouteRequest } from '../middleware/RouteRequest.js'
import { Actions } from '../platform/auth/Auth.js'
import { getApp } from '../app.js'

@RequestRoute("/mcp/audit")
@SignedIn
export class McpAuditController {
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
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_READ } }, async ({ auth }) => {
      const app = await getApp()
      return (await app.mcpAuditEvents.listForOwner(auth.email)).map(McpAuditController.present)
    })
  }

  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_WRITE } }, async ({ auth }) => {
      const app = await getApp()
      return { cleared: await app.mcpAuditEvents.clearForOwner(auth.email) }
    })
  }
}
