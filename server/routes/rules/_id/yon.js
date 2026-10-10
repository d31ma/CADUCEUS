import { Guards } from '../../../middleware/guards.js'
import { Actions } from '../../../platform/auth/bridge.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { RuleValidation } from '../../../features/domains/services/rule-validation.js'
import { normalizeInboxRuleInput } from '../../../features/mailbox/services/rule-input.js'

export class Handler {
  /**
 * DELETE /rules/_id
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async DELETE(/** @type {any} */ { paths, auth }) {
  const ruleId = paths?.id
  if (!ruleId) return Respond.r400('rule id required')

  const app = await getApp()
  const [docId, existing] = await app.rules.findById(ruleId)
  if (!docId || !existing) return Respond.r404('Rule not found')
  if (existing.ownerEmail !== auth.email.toLowerCase()) return Respond.r404('Rule not found')

  await app.rules.delete(docId)
  return { deleted: ruleId }
}

  /**
 * PUT /rules/_id
 * @param {object} params
 * @param {{ name?: string, enabled?: boolean, conditionMatch?: string, conditions?: object[], actions?: object[] }} params.body
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  static async PUT(/** @type {any} */ { paths, body, auth }) {
  const ruleId = paths?.id
  if (!ruleId || !body) return Respond.r400('rule id and body required')

  const app = await getApp()
  const [docId, existing] = await app.rules.findById(ruleId)
  if (!docId || !existing) return Respond.r404('Rule not found')
  if (existing.ownerEmail !== auth.email.toLowerCase()) return Respond.r404('Rule not found')
  const allowed = ['name', 'enabled', 'conditionMatch', 'conditions', 'actions', 'stopProcessing']
  const patch = Object.fromEntries(Object.entries(body).filter(([key]) => allowed.includes(key)))
  const merged = normalizeInboxRuleInput({ ...existing, ...patch }, {
    id: existing.id,
    ownerEmail: existing.ownerEmail ?? auth.email,
    mailbox: existing.mailbox ?? auth.email,
    order: existing.order ?? 0,
  })
  const result = RuleValidation.validateInboxRule(merged)
  if (result.valid === false) return Respond.r400(result.error)
  if (merged.actions.some(action => action.type === 'forward' && action.to?.trim().toLowerCase() === existing.mailbox)) {
    return Respond.r400('A rule cannot forward a message back to its own mailbox')
  }
  await app.rules.update(docId, /** @type {any} */ ({
    name: merged.name,
    enabled: merged.enabled,
    conditionMatch: merged.conditionMatch,
    conditions: merged.conditions,
    actions: merged.actions,
    stopProcessing: merged.stopProcessing,
  }))
  return { updated: ruleId }
}
}
