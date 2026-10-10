import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { getApp } from '../../../app.js'

export class Handler {
  @Guards.auth({ action: Actions.MAILBOX_READ })
  static async GET(/** @type {any} */ { auth }) {
    const app = await getApp()
    return (await app.mcpAuditEvents.listForOwner(auth.email)).map(present)
  }

  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { auth }) {
    const app = await getApp()
    return { cleared: await app.mcpAuditEvents.clearForOwner(auth.email) }
  }
}

function present(record) {
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
