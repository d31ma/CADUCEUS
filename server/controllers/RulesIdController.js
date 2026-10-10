import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { RuleValidation } from '../shared/domains/RuleValidation.js'
import { RuleInput } from '../shared/mailbox/RuleInput.js'

@RequestRoute("/rules/{id}")
@SignedIn
export class RulesIdController {
  /**
 * DELETE /rules/_id
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, auth }) => {
    const ruleId = paths?.id
    if (!ruleId) return Respond.r400('rule id required')

    const app = await getApp()
    const [docId, existing] = await app.rules.findById(ruleId)
    if (!docId || !existing) return Respond.r404('Rule not found')
    if (existing.ownerEmail !== auth.email.toLowerCase()) return Respond.r404('Rule not found')

    await app.rules.delete(docId)
    return { deleted: ruleId }
    })
  }

  /**
 * PUT /rules/_id
 * @param {object} params
 * @param {{ name?: string, enabled?: boolean, conditionMatch?: string, conditions?: object[], actions?: object[] }} params.body
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ paths, body, auth }) => {
    const ruleId = paths?.id
    if (!ruleId || !body) return Respond.r400('rule id and body required')

    const app = await getApp()
    const [docId, existing] = await app.rules.findById(ruleId)
    if (!docId || !existing) return Respond.r404('Rule not found')
    if (existing.ownerEmail !== auth.email.toLowerCase()) return Respond.r404('Rule not found')
    const allowed = ['name', 'enabled', 'conditionMatch', 'conditions', 'actions', 'stopProcessing']
    const patch = Object.fromEntries(Object.entries(body).filter(([key]) => allowed.includes(key)))
    const merged = RuleInput.normalizeInboxRuleInput({ ...existing, ...patch }, {
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
    })
  }
}
